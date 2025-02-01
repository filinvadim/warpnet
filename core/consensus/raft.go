package consensus

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	consensus "github.com/libp2p/go-libp2p-consensus"
	"log"
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
	state         StateCommitter
	raft          *raft.Raft
	fsm           raft.FSM
	config        *raft.Config
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	transport     *raft.NetworkTransport
	consensus     *Consensus
}

// NewRaft TODO
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
	config.Logger = &defaultConsensusLogger{INFO}

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
		stableStore.SetUint64([]byte("CurrentTerm"), 1)
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

	state := ConsensusDefaultState{}
	cons := libp2praft.NewConsensus(&state)
	fsm := cons.FSM()

	transport, err := libp2praft.NewLibp2pTransport(node.Node(), time.Second*60)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft transport: %w", err)
	}
	log.Println("consensus: transport configured with local address:", transport.LocalAddr())

	return &consensusService{
		ctx:           ctx,
		node:          node,
		consRepo:      consRepo,
		state:         NewStateStore(state, cons),
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
	log.Println("consensus: node starting...")

	var err error
	c.raft, err = raft.NewRaft(
		c.config,
		c.fsm,
		c.logStore, c.stableStore,
		c.snapshotStore,
		c.transport,
	)
	if err != nil {
		log.Printf("consensus: failed to create raft node: %v", err)
	}

	bootstrapServers := make([]raft.Server, 0, len(bootstrapAddrs)+1)
	bootstrapServers = append(
		bootstrapServers,
		raft.Server{Suffrage: raft.Voter, ID: c.config.LocalID, Address: c.transport.LocalAddr()},
	)
	for _, addr := range bootstrapAddrs {
		for _, a := range addr.Addrs {
			serverIdP2p := raft.ServerID(addr.ID.String())
			serverAddrP2P := raft.ServerAddress(a.String())
			bootstrapServers = append(
				bootstrapServers,
				raft.Server{Suffrage: raft.Voter, ID: serverIdP2p, Address: serverAddrP2P},
			)
		}
	}

	wait := c.raft.BootstrapCluster(raft.Configuration{Servers: bootstrapServers})
	if wait != nil && wait.Error() != nil {
		log.Printf("consensus: failed to bootstrap cluster: %v", wait.Error())
		return
	}
	go c.listenEvents()
	log.Printf("consensus: node started with ID: %s and last index: %d", c.raft.String(), c.raft.LastIndex())
}

func (c *consensusService) listenEvents() {
	for range c.consensus.Subscribe() {
		log.Println("consensus: state was updated")
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

	for k, v := range newState {
		currentState[k] = v
	}

	updatedState, err := c.consensus.CommitState(currentState)
	return updatedState.(ConsensusDefaultState), err
}

func (c *consensusService) Rollback(badState ConsensusDefaultState) error {
	return c.consensus.Rollback(badState)
}

// TODO
//func (c *consensusService) ReceiveConsensusAppend() {
//	c.consensus...
//}

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
	wait := c.raft.Shutdown()
	if wait != nil && wait.Error() != nil {
		log.Printf("failed to shutdown raft: %v", wait.Error())
	}
	log.Println("consensus: raft node shut down")
	c.raft = nil
}
