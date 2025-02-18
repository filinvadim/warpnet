package consensus

import (
	"context"
	"errors"
	"fmt"
	confFile "github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
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
	raft          *raft.Raft
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	transport     *raft.NetworkTransport
	consensus     *Consensus
	raftConf      raft.Configuration
	isBootstrap   bool
}

func NewRaft(
	ctx context.Context,
	consRepo ConsensusStorer,
	isBootstrap bool,
) (_ *consensusService, err error) {
	var (
		logStore      raft.LogStore
		stableStore   raft.StableStore
		snapshotStore raft.SnapshotStore
	)

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

	_, err = stableStore.Get([]byte("CurrentTerm"))
	if errors.Is(err, database.ErrKeyNotFound) {
		_ = stableStore.SetUint64([]byte("CurrentTerm"), 1)
	}

	last, err := logStore.LastIndex()
	if err != nil {
		return nil, fmt.Errorf("consensus: failed to get last log index: %v", err)
	}

	if last == 0 {
		err = logStore.StoreLog(&raft.Log{
			Index:      1,
			Term:       1,
			Type:       raft.LogCommand,
			Data:       []byte("genesis-log"),
			AppendedAt: time.Now(),
			Extensions: []byte("ext"),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to store genesis log: %v", err)
		}
	}

	cons := libp2praft.NewConsensus(&ConsensusDefaultState{"genesis": ""})

	var (
		bootstrapAddrs, _ = confFile.ConfigFile.Node.AddrInfos()
		bootstrapServers  = make([]raft.Server, 0, len(bootstrapAddrs)+1)
	)
	for _, addr := range bootstrapAddrs {
		serverIdP2p := raft.ServerID(addr.ID.String())
		bootstrapServers = append(
			bootstrapServers,
			raft.Server{Suffrage: raft.Voter, ID: serverIdP2p, Address: raft.ServerAddress(serverIdP2p)},
		)
	}

	return &consensusService{
		ctx:           ctx,
		logStore:      logStore,
		stableStore:   stableStore,
		snapshotStore: snapshotStore,
		consensus:     cons,
		raftConf:      raft.Configuration{Servers: bootstrapServers},
		isBootstrap:   isBootstrap,
	}, nil
}

func (c *consensusService) Negotiate(node NodeServicesProvider) (err error) {
	config := raft.DefaultConfig()
	config.ElectionTimeout = time.Minute
	config.Logger = &defaultConsensusLogger{DEBUG}
	config.LocalID = raft.ServerID(node.ID().String())
	c.transport, err = libp2praft.NewLibp2pTransport(node.Node(), time.Minute)
	if err != nil {
		log.Errorf("failed to create raft transport: %v", err)
		return
	}
	log.Infoln("consensus: transport configured with local address:", c.transport.LocalAddr())

	c.raftConf.Servers = append(
		c.raftConf.Servers,
		raft.Server{Suffrage: raft.Voter, ID: config.LocalID, Address: raft.ServerAddress(config.LocalID)},
	)

	//if c.isBootstrap {
	_ = raft.BootstrapCluster(
		config,
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
		c.raftConf.Clone(),
	)
	//}

	log.Infoln("consensus: raft starting...")

	c.raft, err = raft.NewRaft(
		config,
		c.consensus.FSM(),
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
	)
	if err != nil {
		return fmt.Errorf("consensus: failed to create raft node: %w", err)
	}

	actor := libp2praft.NewActor(c.raft)
	c.consensus.SetActor(actor)

	if !c.isBootstrap {
		err = c.waitForClusterReady(c.raft)
		if err != nil {
			log.Errorf("consensus: cluster did not stabilize: %v", err)
		}
	}

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Errorf("Raft configuration error: %v", err)
	}
	for _, srv := range configFuture.Configuration().Servers {
		log.Infof("Raft peer: ID=%s, Addr=%s", srv.ID, srv.Address)
	}

	peers := node.Node().Peerstore().Peers()
	if len(peers) == 0 {
		log.Warn("No libp2p peers found! Raft cannot form a cluster.")
	} else {
		for _, p := range peers {
			log.Infof("Connected libp2p peer: %s", p)
		}
	}

	log.Infof("consensus: ready  %s and last index: %d", c.raft.String(), c.raft.LastIndex())
	go c.listenEvents()
	return nil
}

func (c *consensusService) waitForClusterReady(r *raft.Raft) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return fmt.Errorf("timed out waiting for cluster to be ready")
		case <-ticker.C:
			if c.ctx.Err() != nil {
				return fmt.Errorf("timed out waiting for cluster to be ready")
			}
			leaderAddr, leaderID := r.LeaderWithID()
			if leaderAddr != "" {
				log.Infof("consensus: raft leader elected: %s", leaderID)
				return nil
			}
			log.Infof("consensus: waiting for leader election...")
		}
	}
}

func (c *consensusService) AddVoter(info warpnet.PeerAddrInfo) {
	if c.raft == nil {
		return
	}
	if info.ID.String() == "" {
		return
	}
	log.Infof("consensus: adding new voter to %s", info.ID.String())
	wait := c.raft.VerifyLeader()
	if wait.Error() != nil {
		log.Infof("consensus: failed to add voter: %s", wait.Error())
		return
	}

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Errorf("consensus: failed to get raft configuration: %v", err)
		return
	}
	prevIndex := configFuture.Index()

	wait = c.raft.RemoveServer(
		raft.ServerID(info.ID.String()), prevIndex, 30*time.Second,
	)
	if err := wait.Error(); err != nil {
		log.Warnf("consensus: failed to remove raft server: %s", wait.Error())
	}

	wait = c.raft.AddVoter(
		raft.ServerID(info.ID.String()), raft.ServerAddress(info.ID.String()), prevIndex, 30*time.Second,
	)
	if wait.Error() != nil {
		log.Errorf("consensus: failed to add voted: %v", wait.Error())
	}
	return
}

func (c *consensusService) RemoveVoter(info warpnet.PeerAddrInfo) {
	if c.raft == nil {
		return
	}
	if info.ID.String() == "" {
		return
	}
	wait := c.raft.VerifyLeader()
	if wait.Error() != nil {
		return
	}

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Errorf("consensus: failed to get raft configuration: %v", err)
		return
	}
	prevIndex := configFuture.Index()

	wait = c.raft.RemoveServer(
		raft.ServerID(info.ID.String()), prevIndex, 30*time.Second,
	)
	if err := wait.Error(); err != nil {
		log.Warnf("consensus: failed to remove raft server: %s", wait.Error())
	}
	return
}

func (c *consensusService) listenEvents() {
	for range c.consensus.Subscribe() {
		log.Infoln("consensus: state was updated")
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
