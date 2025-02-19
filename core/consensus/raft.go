package consensus

import (
	"context"
	"fmt"
	confFile "github.com/filinvadim/warpnet/config"
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
	raft          *raft.Raft
	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	transport     *raft.NetworkTransport
	consensus     *Consensus
	raftConf      raft.Configuration
	raftID        raft.ServerID
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
		path := consRepo.SnapshotPath()
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
		if err != nil {
			return nil, err
		}
		logStore = consRepo
		stableStore = consRepo
		snapshotStore, err = raft.NewFileSnapshotStore(path, 5, f)
		if err != nil {
			log.Fatalf("consensus: failed to create snapshot store: %v", err)
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

	_ = raft.BootstrapCluster(
		config,
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
		c.raftConf.Clone(),
	)

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

	err = c.waitForSync(config.LocalID)
	if err != nil {
		return err
	}
	log.Infof("consensus: ready  %s and last index: %d", c.raft.String(), c.raft.LastIndex())
	return nil
}

func (c *consensusService) waitForSync(id raft.ServerID) error {
	leaderCtx, cancel := context.WithTimeout(c.ctx, 5*time.Minute)
	defer cancel()

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

	err = cs.waitForUpdates()
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
				return nil
			}
			log.Debugf("not voter yet: %s", id)
		}
	}
}

func (c *consensusSync) waitForUpdates() error {
	log.Debugln("raft state is catching up to the latest known version. Please wait...")
	ticker := time.NewTicker(time.Second / 2)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case <-ticker.C:
			lastAppliedIndex := c.raft.AppliedIndex()
			lastIndex := c.raft.LastIndex()
			log.Debugf("current raft index: %d/%d", lastAppliedIndex, lastIndex)
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

func (c *consensusService) CommitState(newState ConsensusDefaultState) (_ ConsensusDefaultState, err error) {
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
