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
	"io"
	"math/rand"
	"sync"
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
	Consensus = libp2praft.Consensus
	State     = consensus.State
)

type ConsensusStorer interface {
	raft.LogStore
	raft.StableStore
	SnapshotFilestore() (file io.Writer, path string)
}

type NodeServicesProvider interface {
	Node() warpnet.P2PNode
	ID() warpnet.WarpPeerID
}

type consensusService struct {
	ctx           context.Context
	consensus     *Consensus
	fsm           *FSM
	raft          *raft.Raft
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	transport     *raft.NetworkTransport
	raftConf      raft.Configuration
	raftID        raft.ServerID
	isBootstrap   bool
	syncMx        *sync.RWMutex
}

func NewRaft(
	ctx context.Context,
	consRepo ConsensusStorer,
	isBootstrap bool,
	validators ...ConsensusValidatorFunc,
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
		f, path := consRepo.SnapshotFilestore()
		logStore = consRepo
		stableStore = consRepo
		snapshotStore, err = raft.NewFileSnapshotStore(path, 5, f)
		if err != nil {
			log.Fatalf("consensus: failed to create snapshot store: %v", err)
		}
	}

	fsm := newFSM(validators...)
	cons := libp2praft.NewConsensus(fsm.state)

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
		fsm:           fsm,
		consensus:     cons,
		raftConf:      raft.Configuration{Servers: bootstrapServers},
		isBootstrap:   isBootstrap,
		syncMx:        new(sync.RWMutex),
	}, nil
}

func (c *consensusService) Sync(node NodeServicesProvider) (err error) {
	c.syncMx.Lock()
	defer c.syncMx.Unlock()

	config := raft.DefaultConfig()
	config.ElectionTimeout = time.Minute
	config.HeartbeatTimeout = time.Second * 30
	config.LeaderLeaseTimeout = time.Second * 30
	config.CommitTimeout = time.Second * 30
	config.LogLevel = "DEBUG"
	config.LocalID = raft.ServerID(node.ID().String())
	c.raftID = raft.ServerID(node.ID().String())

	c.transport, err = libp2praft.NewLibp2pTransport(node.Node(), time.Minute)
	if err != nil {
		log.Errorf("failed to create raft transport: %v", err)
		return
	}

	log.Infoln("consensus: transport configured with local address:", c.transport.LocalAddr())

	err = raft.BootstrapCluster(
		config,
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
		c.raftConf.Clone(),
	)
	if err != nil {
		// random wait to avoid cluster nodes simultaneous setup
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(500)))

		termKey := []byte("CurrentTerm")
		if term, _ := c.stableStore.GetUint64(termKey); term == 0 {
			_ = c.stableStore.SetUint64(termKey, 1)
		}
		err := c.logStore.GetLog(2, &raft.Log{})
		if errors.Is(err, database.ErrConsensusKeyNotFound) {
			entry := &raft.Log{
				Type:  raft.LogConfiguration,
				Index: 1,
				Term:  1,
				Data:  raft.EncodeConfiguration(c.raftConf),
			}
			_ = c.logStore.StoreLog(entry)
		}
	}

	log.Infoln("consensus: raft starting...")

	c.raft, err = raft.NewRaft(
		config,
		c.fsm,
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

	err = c.sync(config.LocalID)
	if err != nil {
		return err
	}
	log.Infof("consensus: ready  %s and last index: %d", c.raft.String(), c.raft.LastIndex())
	return nil
}

func (c *consensusService) sync(id raft.ServerID) error {
	if c.raftID == "" {
		panic("consensus: raft id not initialized")
	}
	leaderCtx, leaderCancel := context.WithTimeout(c.ctx, time.Minute)
	defer leaderCancel()

	cs := consensusSync{
		ctx:    c.ctx,
		raft:   c.raft,
		raftID: id,
	}

	leaderID, err := cs.waitForLeader(leaderCtx)
	if err != nil {
		return fmt.Errorf("waiting for leader: %w", err)
	}

	log.Infof("current raft leader: %s", leaderID)

	err = cs.waitForVoter()
	if err != nil {
		return fmt.Errorf("waiting to become a voter: %w", err)
	}

	updatesCtx, updatesCancel := context.WithTimeout(c.ctx, time.Minute)
	defer updatesCancel()

	err = cs.waitForUpdates(updatesCtx)
	if err != nil {
		return fmt.Errorf("waiting for consensus updates: %w", err)
	}
	return nil
}

type consensusSync struct {
	ctx    context.Context
	raft   *raft.Raft
	raftID raft.ServerID
}

func (c *consensusSync) waitForLeader(ctx context.Context) (string, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if addr, id := c.raft.LeaderWithID(); addr != "" {
				return string(id), nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (c *consensusSync) waitForVoter() error {
	log.Debugln("waiting until we are promoted to a voter")
	ticker := time.NewTicker(time.Second / 2)
	defer ticker.Stop()

	id := c.raftID
	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case <-ticker.C:
			wait := c.raft.GetConfiguration()
			if err := wait.Error(); err != nil {
				return err
			}

			if isVoter(id, wait.Configuration()) {
				log.Infof("consensus: node is voter: %s", id)
				return nil
			}
			log.Debugf("not voter yet: %s", id)
		}
	}
}

func (c *consensusSync) waitForUpdates(ctx context.Context) error {
	log.Debugln("raft state is catching up to the latest known version. Please wait...")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			lastAppliedIndex := c.raft.AppliedIndex()
			lastIndex := c.raft.LastIndex()
			log.Infof("current raft index: %d/%d", lastAppliedIndex, lastIndex)
			if lastAppliedIndex == lastIndex {
				return nil
			}
		}
	}
}

func isVoter(srvID raft.ServerID, cfg raft.Configuration) bool {
	for _, server := range cfg.Servers {
		if server.ID == srvID && server.Suffrage == raft.Voter {
			return true
		}
	}
	return false
}

func (c *consensusService) AddVoter(info warpnet.PeerAddrInfo) {
	if c.raft == nil {
		return
	}
	if info.ID.String() == "" {
		return
	}

	c.waitSync()

	if _, leaderId := c.raft.LeaderWithID(); c.raftID != leaderId {
		return
	}
	log.Infof("consensus: adding new voter %s", info.ID.String())

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Errorf("consensus: failed to get raft configuration: %v", err)
		return
	}
	prevIndex := configFuture.Index()

	wait := c.raft.RemoveServer(
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

	c.waitSync()

	if _, leaderId := c.raft.LeaderWithID(); c.raftID != leaderId {
		return
	}
	log.Infof("consensus: removing voter %s", info.ID.String())

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Errorf("consensus: failed to get raft configuration: %v", err)
		return
	}
	prevIndex := configFuture.Index()

	wait := c.raft.RemoveServer(
		raft.ServerID(info.ID.String()), prevIndex, 30*time.Second,
	)
	if err := wait.Error(); err != nil {
		log.Warnf("consensus: failed to remove raft server: %s", wait.Error())
	}
	return
}

func (c *consensusService) LeaderID() string {
	_, leaderId := c.raft.LeaderWithID()
	return string(leaderId)

}

func (c *consensusService) CommitState(newState KVState) (_ *KVState, err error) {
	if c.raft == nil {
		return nil, errors.New("consensus: nil raft service")
	}

	c.waitSync()

	if _, leaderId := c.raft.LeaderWithID(); c.raftID != leaderId {
		log.Warnf("not a leader: %s", leaderId)
		return nil, nil
	}

	returnedState, err := c.consensus.CommitState(newState)
	if err != nil {
		return nil, err
	}
	if kvState, ok := returnedState.(*KVState); ok {
		return kvState, nil
	}

	if err, ok := returnedState.(error); ok {
		return nil, err
	}

	return nil, fmt.Errorf("consensus: failed to commit state: %v", returnedState)
}

func (c *consensusService) CurrentState() (*KVState, error) {
	if c.raft == nil {
		return nil, errors.New("consensus: nil raft service")
	}

	c.waitSync()

	currentState, err := c.consensus.GetCurrentState()
	if err != nil {
		return nil, fmt.Errorf("consensus: get: failed to get current state: %v", err)
	}
	defaultState, ok := currentState.(*KVState)
	if !ok {
		return nil, fmt.Errorf("consensus: get: failed to assert state type")
	}
	return defaultState, nil
}

func (c *consensusService) waitSync() {
	c.syncMx.RLock()
	c.syncMx.RUnlock()
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
