package consensus

import (
	"context"
	"errors"
	"fmt"
	confFile "github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/warpnet"
	ipfslog "github.com/ipfs/go-log/v2"
	consensus "github.com/libp2p/go-libp2p-consensus"
	log "github.com/sirupsen/logrus"
	"io"
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
	fsm           *fsm
	raft          *raft.Raft
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	transport     *raft.NetworkTransport
	raftConf      raft.Configuration
	raftID        raft.ServerID
	syncMx        *sync.RWMutex
}

func NewRaft(
	ctx context.Context,
	consRepo ConsensusStorer,
	isBootstrap bool,
	validators ...ConsensusValidatorFunc,
) (_ *consensusService, err error) {
	ipfslog.SetLogLevel("raftlib", "DEBUG")

	var stableStore raft.StableStore = raft.NewInmemStore()
	var snapshotStore raft.SnapshotStore = raft.NewInmemSnapshotStore()
	if !isBootstrap {
		f, path := consRepo.SnapshotFilestore()
		stableStore = consRepo
		snapshotStore, err = raft.NewFileSnapshotStore(path, 5, f)
		if err != nil {
			log.Fatalf("consensus: failed to create snapshot store: %v", err)
		}
	}

	finiteStateMachine := newFSM(validators...)
	cons := libp2praft.NewConsensus(finiteStateMachine.state)

	var (
		bootstrapAddrs, _ = confFile.ConfigFile.Node.AddrInfos()
		bootstrapServers  = make([]raft.Server, 0, len(bootstrapAddrs))
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
		logStore:      raft.NewInmemStore(),
		stableStore:   stableStore,
		snapshotStore: snapshotStore,
		fsm:           finiteStateMachine,
		consensus:     cons,
		raftConf:      raft.Configuration{Servers: bootstrapServers},
		syncMx:        new(sync.RWMutex),
	}, nil
}

func (c *consensusService) Sync(node NodeServicesProvider) (err error) {
	c.syncMx.Lock()
	defer c.syncMx.Unlock()

	config := raft.DefaultConfig()
	config.ElectionTimeout = time.Minute
	config.HeartbeatTimeout = time.Minute
	config.LeaderLeaseTimeout = time.Second * 10
	config.CommitTimeout = time.Minute
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
		log.Errorf("failed to bootstrap cluster: %v", err)
	}

	//if err = c.logStore.GetLog(1, &raft.Log{}); errors.Is(err, database.ErrConsensusKeyNotFound) {
	//	c.logStore.StoreLog(&raft.Log{
	//		Type: raft.LogConfiguration, Index: 1, Term: 1,
	//		Data: raft.EncodeConfiguration(c.raftConf),
	//	})
	//}

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

	err = c.sync()
	if err != nil {
		return err
	}
	log.Infof("consensus: ready %s and last index: %d", c.raft.String(), c.raft.LastIndex())
	return nil
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

	log.Infof("consensus: current raft leader: %s", leaderID)

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
