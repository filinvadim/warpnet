package consensus

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/warpnet"
	consensus "github.com/libp2p/go-libp2p-consensus"
	log "github.com/sirupsen/logrus"
	"os"
	"time"

	"github.com/hashicorp/raft"
	"github.com/libp2p/go-libp2p-raft"
)

/*
	Raft is a consensus algorithm designed for managing replicated logs in distributed systems.
	It was developed as a more understandable alternative to Paxos and is used to ensure data consistency across nodes.

  Raft solves three key tasks:
  1. **Leader Election**: One node is elected as the leader, responsible for managing log entries.
  2. **Log Replication**: The leader accepts commands and distributes them to other nodes for synchronization.
  3. **Safety and Fault Tolerance**: Ensures that data remains consistent even in the event of failures.

  Raft provides **strong consistency**, making it suitable for distributed systems that require predictability
  and protection against network partitioning.

  The **go-libp2p-consensus** library is a module for libp2p that enables the integration of consensus mechanisms
  (including Raft) into peer-to-peer (P2P) networks. It provides an abstract interface that can be implemented
  for various consensus algorithms, including Raft, PoW, PoS, and BFT-based systems.

  ### **Key Features of go-libp2p-consensus:**
  - **Consensus Algorithm Abstraction**
    - Supports Raft and other algorithms (e.g., PoS).
  - **Integration with libp2p**
    - Designed for decentralized systems without a central coordinator.
  - **Flexibility**
    - Developers can implement custom consensus logic by extending the library's interfaces.
  - **Optimized for P2P Environments**
    - Unlike traditional Raft, it is adapted for dynamically changing networks.
*/

type (
	ConsensusDefaultState map[string]string
	Consensus             = libp2praft.Consensus
	State                 = consensus.State
)

type ConsensusStorer interface {
	raft.LogStore
	raft.StableStore
	SnapshotPath() string
}

type StateCommitter interface {
	Commit(key string, secret string) (err error)
}

type NodeServicesProvider interface {
	Node() warpnet.P2PNode
	ID() warpnet.WarpPeerID
}

type consensusService struct {
	ctx           context.Context
	node          NodeServicesProvider
	consRepo      ConsensusStorer
	raft          *raft.Raft
	fsm           raft.FSM
	config        *raft.Config
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	transport     *raft.NetworkTransport
	consensus     *Consensus
}

func NewRaft(
	ctx context.Context,
	node NodeServicesProvider,
	consRepo ConsensusStorer,
	isBootstrap bool,
) (_ *consensusService, err error) {
	var (
		logStore      raft.LogStore
		stableStore   raft.StableStore
		snapshotStore raft.SnapshotStore
	)

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(node.ID().String())
	config.ElectionTimeout = 60 * time.Second
	config.Logger = &defaultConsensusLogger{DEBUG}

	if isBootstrap {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
		snapshotStore = raft.NewInmemSnapshotStore()
	} else {
		logStore = consRepo
		stableStore = consRepo
		snapshotStore, err = raft.NewFileSnapshotStore(consRepo.SnapshotPath(), 5, os.Stdout)
		if err != nil {
			log.Fatalf("consensus: failed to create snapshot store: %v", err)
		}
	}

	cons := libp2praft.NewConsensus(&ConsensusDefaultState{"genesis": ""})
	fsm := cons.FSM()

	transport, err := libp2praft.NewLibp2pTransport(node.Node(), time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft transport: %w", err)
	}
	log.Infoln("consensus: transport configured with local address:", transport.LocalAddr())

	return &consensusService{
		ctx:           ctx,
		node:          node,
		consRepo:      consRepo,
		fsm:           fsm,
		config:        config,
		logStore:      logStore,
		stableStore:   stableStore,
		snapshotStore: snapshotStore,
		transport:     transport,
		consensus:     cons,
	}, nil
}

func (c *consensusService) Negotiate(bootstrapAddrs []warpnet.PeerAddrInfo) {
	log.Infoln("consensus: raft starting...")

	bootstrapServers := make([]raft.Server, 0, len(bootstrapAddrs)+1)
	bootstrapServers = append(
		bootstrapServers,
		raft.Server{Suffrage: raft.Voter, ID: c.config.LocalID, Address: raft.ServerAddress(c.config.LocalID)},
	)
	for _, addr := range bootstrapAddrs {
		serverIdP2p := raft.ServerID(addr.ID.String())
		bootstrapServers = append(
			bootstrapServers,
			raft.Server{Suffrage: raft.Voter, ID: serverIdP2p, Address: raft.ServerAddress(serverIdP2p)},
		)
	}
	serversConfig := raft.Configuration{Servers: bootstrapServers}

	err := raft.BootstrapCluster(
		c.config,
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
		serversConfig.Clone(),
	)
	// ErrCantBootstrap means that cluster was already bootstrapped
	if err != nil && !errors.Is(err, raft.ErrCantBootstrap) {
		log.Errorf("consensus: failed to bootstrap cluster: %v", err)
		return
	}

	c.raft, err = raft.NewRaft(
		c.config,
		c.fsm,
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
	)
	if err != nil {
		log.Infof("consensus: failed to create raft node: %v", err)
		return
	}
	actor := libp2praft.NewActor(c.raft)
	c.consensus.SetActor(actor)

	waitForLeader(c.raft)
	go c.listenEvents()
	log.Infof("consensus: started  %s and last index: %d", c.raft.String(), c.raft.LastIndex())
}

func (c *consensusService) listenEvents() {
	for range c.consensus.Subscribe() {
		log.Infoln("consensus: state was updated")
	}
}

// New Raft does not allow leader observation directy
// What's worse, there will be no notification that a new
// leader was elected because observations are set before
// setting the Leader and only when the RaftState has changed.
// Therefore, we need a ticker.
func waitForLeader(r *raft.Raft) {
	obsCh := make(chan raft.Observation, 1)
	observer := raft.NewObserver(obsCh, false, nil)
	r.RegisterObserver(observer)
	defer r.DeregisterObserver(observer)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case obs := <-obsCh:
			_, ok := obs.Data.(raft.RaftState)
			if !ok {
				continue
			}
			if leaderAddr, _ := r.LeaderWithID(); leaderAddr != "" {
				log.Infof("consensus: raft leader found with address: %s", leaderAddr)
				return
			}

		case <-ticker.C:
			if leaderAddr, _ := r.LeaderWithID(); leaderAddr != "" {
				log.Infof("consensus: raft leader found with address: %s", leaderAddr)
				return
			}
		case <-ctx.Done():
			log.Errorln("timed out waiting for raft leader")
			return
		}
	}
}

func (c *consensusService) Commit(newState ConsensusDefaultState) (_ ConsensusDefaultState, err error) {
	state, err := c.consensus.GetCurrentState()
	if err != nil {
		return nil, fmt.Errorf("consensus: commit: failed to get current state: %v", err)
	}
	currentState, ok := state.(ConsensusDefaultState)
	if !ok {
		return nil, fmt.Errorf("consensus: commit: failed to assert state type")
	}

	updatedState := make(ConsensusDefaultState)
	for k, v := range currentState {
		updatedState[k] = v
	}
	for k, v := range newState {
		updatedState[k] = v
	}

	returnedState, err := c.consensus.CommitState(updatedState)
	return returnedState.(ConsensusDefaultState), err
}

func (c *consensusService) CurrentState() (ConsensusDefaultState, error) {
	currentState, err := c.consensus.GetCurrentState()
	if err != nil {
		return nil, fmt.Errorf("consensus: get: failed to get current state: %v", err)
	}
	defaultState, ok := currentState.(ConsensusDefaultState)
	if !ok {
		return nil, fmt.Errorf("consensus: get: failed to assert state type")
	}
	return defaultState, nil
}

func (c *consensusService) Shutdown() {
	if c == nil || c.raft == nil {
		return
	}
	_ = c.transport.Close()
	wait := c.raft.Shutdown()
	if wait != nil && wait.Error() != nil {
		log.Infof("failed to shutdown raft: %v", wait.Error())
	}
	log.Infoln("consensus: raft node shut down")
	c.raft = nil
}
