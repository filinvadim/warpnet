package consensus

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	consensus "github.com/libp2p/go-libp2p-consensus"
	log "github.com/sirupsen/logrus"
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
var ErrNoRaftCluster = errors.New("consensus: no cluster found")

type (
	Consensus = libp2praft.Consensus
	State     = consensus.State
)

type ConsensusStorer interface {
	raft.StableStore
	Path() string
}

type NodeServicesProvider interface {
	Node() warpnet.P2PNode
	NodeInfo() warpnet.NodeInfo
	GenericStream(nodeIdStr string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type votersCacher interface {
	addVoter(key raft.ServerID, srv raft.Server)
	getVoter(key raft.ServerID) (_ raft.Server, err error)
	removeVoter(key raft.ServerID)
	print()
	close()
}

type Streamer interface {
	NodeInfo() warpnet.NodeInfo
	GenericStream(nodeIdStr string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type consensusService struct {
	ctx           context.Context
	consensus     *Consensus
	streamer      Streamer
	fsm           *fsm
	cache         votersCacher
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
		snapshotStore, err = raft.NewFileSnapshotStore("/tmp/snapshot", 5, os.Stderr)
		if err != nil {
			log.Fatalf("consensus: failed to create snapshot store: %v", err)
		}
		stableStore = raft.NewInmemStore()
	} else {
		stableStore = consRepo
		snapshotStore, err = raft.NewFileSnapshotStore(consRepo.Path(), 5, os.Stderr)
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
		cache:         newVotersCache(),
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
	config.Logger = newConsensusLogger("error", "consensus")
	config.LocalID = raft.ServerID(node.NodeInfo().ID.String())
	config.NoLegacyTelemetry = true
	config.SnapshotThreshold = 8192
	config.SnapshotInterval = 20 * time.Second
	config.NoSnapshotRestoreOnStart = true

	if err := raft.ValidateConfig(config); err != nil {
		return err
	}
	c.raftID = config.LocalID

	c.transport, err = libp2praft.NewLibp2pTransport(node.Node(), time.Minute)
	if err != nil {
		log.Errorf("failed to create node transport: %v", err)
		return
	}
	log.Infoln("consensus: transport configured with local address:", c.transport.LocalAddr())

	if err := c.forceBootstrap(config.LocalID); err != nil {
		return err
	}

	log.Infof("consensus: node %s starting...", c.raftID)
	c.raft, err = raft.NewRaft(
		config,
		c.fsm,
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
	)
	if err != nil {
		return fmt.Errorf("consensus: failed to create node: %w", err)
	}

	wait := c.raft.GetConfiguration()
	if err := wait.Error(); err != nil {
		log.Errorf("consensus: node configuration error: %v", err)
	}

	c.consensus.SetActor(libp2praft.NewActor(c.raft))

	err = c.sync()
	if err != nil {
		return err
	}
	log.Infof("consensus: ready node %s with last index: %d", c.raftID, c.raft.LastIndex())
	c.streamer = node
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

	// force Raft to create a cluster no matter what
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
		panic("consensus: node id is not initialized")
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
		log.Errorf("waiting for leader: %v", err)
	}

	if string(c.raftID) == leaderID {
		log.Infoln("consensus: node is a leader!")
	} else {
		log.Infof("consensus: current leader: %s", leaderID)
	}

	log.Infoln("consensus: waiting until we are promoted to a voter...")
	voterCtx, voterCancel := context.WithTimeout(c.ctx, time.Minute)
	defer voterCancel()

	if err = cs.waitForVoter(voterCtx); err != nil {
		return fmt.Errorf("consensus: waiting to become a voter: %w", err)
	}
	log.Infoln("consensus: node received voter status")

	updatesCtx, updatesCancel := context.WithTimeout(c.ctx, time.Minute*2)
	defer updatesCancel()

	if err = cs.waitForUpdates(updatesCtx); err != nil {
		return fmt.Errorf("consensus: waiting for consensus updates: %w", err)
	}

	wait := c.raft.GetConfiguration()
	if err := wait.Error(); err != nil {
		return err
	}

	for _, srv := range wait.Configuration().Servers {
		c.cache.addVoter(srv.ID, srv)
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
	ticker := time.NewTicker(time.Second)
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
			log.Debugf("consensus: node is not voter yet: %s", id)
		}
	}
}

func (c *consensusSync) waitForUpdates(ctx context.Context) error {
	log.Debugln("consensus: node state is catching up to the latest known version. Please wait...")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			lastAppliedIndex := c.raft.AppliedIndex()
			lastIndex := c.raft.LastIndex()
			log.Infof("consensus: current node index: %d/%d", lastAppliedIndex, lastIndex)
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
			log.Infof("consensus: node promoted to %s", server.Suffrage)
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

	id := raft.ServerID(info.ID.String())
	addr := raft.ServerAddress(info.ID.String())

	if _, err := c.cache.getVoter(id); err == nil {
		return
	}
	log.Infof("consensus: adding new voter %s", info.ID.String())

	wait := c.raft.AddVoter(id, addr, 0, 30*time.Second)
	if wait.Error() != nil {
		log.Errorf("consensus: failed to add voted: %v", wait.Error())
	}

	c.cache.addVoter(id, raft.Server{
		Suffrage: raft.Voter,
		ID:       id,
		Address:  addr,
	})
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

	if _, err := c.cache.getVoter(raft.ServerID(id.String())); errors.Is(err, errVoterNotFound) {
		return
	}
	log.Infof("consensus: removing voter %s", id.String())

	wait := c.raft.RemoveServer(raft.ServerID(id.String()), 0, 30*time.Second)
	if err := wait.Error(); err != nil {
		log.Errorf("consensus: failed to remove node: %s", wait.Error())
		return
	}
	c.cache.removeVoter(raft.ServerID(id.String()))
	return
}

func (c *consensusService) Stats() map[string]string {
	s := c.raft.Stats()
	return map[string]string{
		"election_state":  s["state"],
		"election_period": s["term"],
		"commit_index":    s["commit_index"],
		"applied_index":   s["applied_index"],
		"fsm_pending":     s["fsm_pending"],
		"last_contact":    s["last_contact"],
	}
}

func (c *consensusService) LeaderID() warpnet.WarpPeerID {
	_, leaderId := c.raft.LeaderWithID()
	return warpnet.FromStringToPeerID(string(leaderId))
}

func (c *consensusService) AskUserValidation(user domain.User) error {
	log.Infoln("consensus: asking for user validation...")

	bt, err := json.JSON.Marshal(user)
	if err != nil {
		return err
	}
	newState := map[string]string{
		database.UserConsensusKey: string(bt),
	}

	leaderId := c.LeaderID().String()
	if leaderId == string(c.raftID) {
		_, err := c.CommitState(newState)
		if errors.Is(err, ErrNoRaftCluster) {
			return nil
		}
		return fmt.Errorf("consensus: failed to commit validate user state: %w", err)
	}

	resp, err := c.streamer.GenericStream(leaderId, event.PUBLIC_POST_NODE_VERIFY, newState)
	if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
		return fmt.Errorf("consensus: node verify stream: %w", err)
	}
	if len(resp) == 0 {
		return nil
	}

	var errResp event.ErrorResponse
	if _ = json.JSON.Unmarshal(resp, &errResp); errResp.Message != "" {
		log.Errorf("consensus: verify response unmarshal failed: %v", errResp)
		return fmt.Errorf("consensus: verify response unmarshal failed: %w", errResp)
	}

	updatedState := make(map[string]string)
	if err = json.JSON.Unmarshal(resp, &updatedState); err != nil {
		log.Errorf("consensus: failed to unmarshal updated consensus state %s: %v", resp, err)
		return ErrConsensusRejection
	}

	log.Infoln("consensus: user validated")
	return nil
}

func (c *consensusService) CommitState(newState KVState) (_ *KVState, err error) {
	if c.raft == nil {
		return nil, errors.New("consensus: nil node")
	}

	c.waitSync()

	wait := c.raft.GetConfiguration()
	if len(wait.Configuration().Servers) <= 1 {
		return nil, ErrNoRaftCluster
	}

	if _, leaderId := c.raft.LeaderWithID(); c.raftID != leaderId {
		log.Warnf("consensus: not a leader: %s", leaderId)
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
		return nil, errors.New("consensus: nil node")
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
		log.Infof("consensus: failed to shutdown node: %v", wait.Error())
	}
	c.cache.close()
	c.raft = nil
	log.Infoln("consensus: node shut down")

}
