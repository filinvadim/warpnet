package consensus

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/warpnet"
	consensus "github.com/libp2p/go-libp2p-consensus"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
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
	raft.StableStore
	SnapshotFilestore() (file io.Writer, path string)
}

type NodeServicesProvider interface {
	Node() warpnet.P2PNode
	NodeInfo() warpnet.NodeInfo
}

type consensusService struct {
	ctx           context.Context
	consensus     *Consensus
	fsm           *fsm
	raft          *raft.Raft
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	transport     *raft.NetworkTransport
	raftID        raft.ServerID
	syncMx        *sync.RWMutex
}

func NewBootstrapRaft(ctx context.Context, validators ...ConsensusValidatorFunc) (_ *consensusService, err error) {
	return NewRaft(ctx, nil, true, validators...)
}

func NewRaft(
	ctx context.Context,
	consRepo ConsensusStorer,
	isBootstrap bool,
	validators ...ConsensusValidatorFunc,
) (_ *consensusService, err error) {
	var (
		stableStore   raft.StableStore
		snapshotStore raft.SnapshotStore
	)

	if isBootstrap {
		path := "/tmp/snapshot/snapshot"
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
		if err != nil {
			return nil, err
		}
		snapshotStore, err = raft.NewFileSnapshotStore("/tmp/snapshot", 5, f)
		if err != nil {
			log.Fatalf("consensus: failed to create snapshot store: %v", err)
		}
		stableStore = raft.NewInmemStore()
	} else {
		stableStore = consRepo
		f, path := consRepo.SnapshotFilestore()
		snapshotStore, err = raft.NewFileSnapshotStore(path, 5, f)
		if err != nil {
			log.Fatalf("consensus: failed to create snapshot store: %v", err)
		}
	}

	finiteStateMachine := newFSM(validators...)
	cons := libp2praft.NewConsensus(finiteStateMachine.state)

	return &consensusService{
		ctx:           ctx,
		logStore:      raft.NewInmemStore(),
		stableStore:   stableStore,
		snapshotStore: snapshotStore,
		fsm:           finiteStateMachine,
		consensus:     cons,
		syncMx:        new(sync.RWMutex),
	}, nil
}

func (c *consensusService) Sync(node NodeServicesProvider) (err error) {
	c.syncMx.Lock()
	defer c.syncMx.Unlock()

	config := raft.DefaultConfig()
	config.HeartbeatTimeout = time.Second * 5
	config.ElectionTimeout = config.HeartbeatTimeout
	config.LeaderLeaseTimeout = config.HeartbeatTimeout
	config.CommitTimeout = time.Second * 30
	config.LogLevel = "ERROR"
	config.LocalID = raft.ServerID(node.NodeInfo().ID.String())
	config.NoLegacyTelemetry = true
	config.SnapshotThreshold = 50
	config.NoSnapshotRestoreOnStart = true
	config.SnapshotInterval = time.Hour

	if err := raft.ValidateConfig(config); err != nil {
		return err
	}
	c.raftID = config.LocalID

	c.transport, err = libp2praft.NewLibp2pTransport(node.Node(), time.Minute)
	if err != nil {
		log.Errorf("failed to create raft transport: %v", err)
		return
	}
	log.Infoln("consensus: transport configured with local address:", c.transport.LocalAddr())

	if err := c.forceBootstrap(config.LocalID); err != nil {
		return err
	}

	log.Infof("consensus: raft server %s starting...", c.raftID)
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

	wait := c.raft.GetConfiguration()
	if err := wait.Error(); err != nil {
		log.Errorf("consensus: raft configuration error: %v", err)
	}

	c.consensus.SetActor(libp2praft.NewActor(c.raft))

	err = c.sync()
	if err != nil {
		return err
	}
	log.Infof("consensus: ready node %s with last index: %d", c.raftID, c.raft.LastIndex())
	return nil
}

func (c *consensusService) forceBootstrap(id raft.ServerID) error {

	lastIndex, err := c.logStore.LastIndex()
	if err != nil {
		return fmt.Errorf("consensus: failed to read last log index: %v", err)
	}

	if lastIndex != 0 {
		return nil
	}
	log.Infoln("consensus: bootstrapping a new cluster with server id:", id)

	// force Raft to create cluster no matter what
	raftConf := raft.Configuration{}
	raftConf.Servers = append(raftConf.Servers, raft.Server{
		Suffrage: raft.Voter,
		ID:       id,
		Address:  raft.ServerAddress(id),
	})

	if err := c.stableStore.SetUint64([]byte("CurrentTerm"), 1); err != nil {
		return fmt.Errorf("consensus: failed to save current term: %v", err)
	}
	if err := c.logStore.StoreLog(&raft.Log{
		Type: raft.LogConfiguration, Index: 1, Term: 1,
		Data: raft.EncodeConfiguration(raftConf),
	}); err != nil {
		return fmt.Errorf("consensus: failed to store bootstrap log: %v", err)
	}

	return c.logStore.GetLog(1, &raft.Log{})
}

type consensusSync struct {
	raft   *raft.Raft
	raftID raft.ServerID
}

func (c *consensusService) sync() error {
	if c.raftID == "" {
		panic("consensus: raft id not initialized")
	}

	leaderCtx, leaderCancel := context.WithTimeout(c.ctx, time.Minute)
	defer leaderCancel()

	cs := consensusSync{
		raft:   c.raft,
		raftID: c.raftID,
	}

	log.Infoln("consensus: waiting for leader...")
	leaderID, err := cs.waitForLeader(leaderCtx)
	if err != nil {
		return fmt.Errorf("waiting for leader: %w", err)
	}

	if string(c.raftID) == leaderID {
		log.Infoln("consensus: node is a leader!")
	} else {
		log.Infof("consensus: current leader: %s", leaderID)
	}

	log.Infoln("consensus: waiting until we are promoted to a voter...")
	voterCtx, voterCancel := context.WithTimeout(c.ctx, time.Minute)
	defer voterCancel()

	err = cs.waitForVoter(voterCtx)
	if err != nil {
		return fmt.Errorf("consensus: waiting to become a voter: %w", err)
	}
	log.Infoln("consensus: node received voter status")

	updatesCtx, updatesCancel := context.WithTimeout(c.ctx, time.Minute*5)
	defer updatesCancel()

	err = cs.waitForUpdates(updatesCtx)
	if err != nil {
		return fmt.Errorf("consensus: waiting for consensus updates: %w", err)
	}
	log.Infoln("consensus: sync complete")
	return nil
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

func (c *consensusSync) waitForVoter(ctx context.Context) error {
	ticker := time.NewTicker(time.Second / 2)
	defer ticker.Stop()

	id := c.raftID
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			wait := c.raft.GetConfiguration()
			if err := wait.Error(); err != nil {
				return err
			}

			if isVoter(id, wait.Configuration()) {
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
		if server.ID == srvID {
			log.Infof("consensus: raft server is a %s", server.Suffrage)
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

	id := raft.ServerID(info.ID.String())
	addr := raft.ServerAddress(info.ID.String())

	wait := c.raft.RemoveServer(id, 0, 30*time.Second)
	if err := wait.Error(); err != nil {
		log.Warnf("consensus: failed to remove raft server: %s", wait.Error())
	}

	wait = c.raft.AddVoter(id, addr, 0, 30*time.Second)
	if wait.Error() != nil {
		log.Errorf("consensus: failed to add voted: %v", wait.Error())
	}
	return
}

func (c *consensusService) RemoveVoter(id warpnet.WarpPeerID) {
	if c.raft == nil {
		return
	}
	if id.String() == "" {
		return
	}

	c.waitSync()

	if _, leaderId := c.raft.LeaderWithID(); c.raftID != leaderId {
		return
	}
	log.Infof("consensus: removing voter %s", id.String())

	configFuture := c.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Errorf("consensus: failed to get raft configuration: %v", err)
		return
	}

	wait := c.raft.RemoveServer(raft.ServerID(id.String()), 0, 30*time.Second)
	if err := wait.Error(); err != nil {
		log.Warnf("consensus: failed to remove raft server: %s", wait.Error())
	}
	return
}

func (c *consensusService) LeaderID() warpnet.WarpPeerID {
	_, leaderId := c.raft.LeaderWithID()

	return warpnet.FromStringToPeerID(string(leaderId))
}

var ErrNoRaftCluster = errors.New("no raft cluster found")

func (c *consensusService) CommitState(newState KVState) (_ *KVState, err error) {
	if c.raft == nil {
		return nil, errors.New("consensus: nil raft service")
	}

	c.waitSync()

	wait := c.raft.GetConfiguration()
	if len(wait.Configuration().Servers) <= 1 {
		return nil, ErrNoRaftCluster
	}

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
