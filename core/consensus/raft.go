package consensus

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/retrier"
	consensus "github.com/libp2p/go-libp2p-consensus"
	log "github.com/sirupsen/logrus"
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
	    - Unlike the traditional Raft, it is adapted for dynamically changing networks.
*/

const initiatorServerID raft.ServerID = "12D3KooWMKZFrp1BDKg9amtkv5zWnLhuUXN32nhqMvbtMdV2hz7j"

var ErrNoRaftCluster = errors.New("consensus: no cluster found")

type (
	Consensus = libp2praft.Consensus
	State     = consensus.State
)

type ConsensusStorer interface {
	raft.StableStore
	SnapshotsPath() string
}

type NodeTransporter interface {
	Node() warpnet.P2PNode
	NodeInfo() warpnet.NodeInfo
	Network() warpnet.WarpNetwork
	GenericStream(nodeIdStr string, path stream.WarpRoute, data any) (_ []byte, err error)
}

type votersCacher interface {
	addVoter(key raft.ServerID, srv raft.Server)
	getVoter(key raft.ServerID) (_ raft.Server, err error)
	removeVoter(key raft.ServerID) error
	print()
	close()
}

type consensusService struct {
	ctx            context.Context
	consensus      *Consensus
	node           NodeTransporter
	fsm            *fsm
	cache          votersCacher
	raft           *raft.Raft
	logStore       raft.LogStore
	stableStore    raft.StableStore
	snapshotStore  raft.SnapshotStore
	transport      *raft.NetworkTransport
	raftID         raft.ServerID
	syncMx         *sync.RWMutex
	retrier        retrier.Retrier
	l              *consensusLogger
	bootstrapNodes []warpnet.PeerAddrInfo
	isPrivate      bool
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

	infos, err := config.ConfigFile.Node.AddrInfos()
	if err != nil {
		return nil, err
	}

	l := newConsensusLogger(log.ErrorLevel.String(), "raft")

	if isBootstrap {
		basePath := "/tmp/snapshot"
		snapshotStore, err = raft.NewFileSnapshotStoreWithLogger(basePath, 5, l)
		if err != nil {
			return nil, fmt.Errorf("consensus: failed to create snapshot store: %v", err)
		}
		stableStore = raft.NewInmemStore()
	} else {
		stableStore = consRepo
		snapshotStore, err = raft.NewFileSnapshotStoreWithLogger(consRepo.SnapshotsPath(), 5, l)
		if err != nil {
			return nil, fmt.Errorf("consensus: failed to create snapshot store: %v", err)
		}
	}

	finiteStateMachine := newFSM(validators...)
	cons := libp2praft.NewConsensus(finiteStateMachine.state)

	return &consensusService{
		ctx:            ctx,
		logStore:       raft.NewInmemStore(),
		stableStore:    stableStore,
		snapshotStore:  snapshotStore,
		fsm:            finiteStateMachine,
		cache:          newVotersCache(),
		consensus:      cons,
		syncMx:         new(sync.RWMutex),
		retrier:        retrier.New(time.Second*3, 3, retrier.ArithmeticalBackoff),
		l:              l,
		isPrivate:      !isBootstrap,
		bootstrapNodes: infos,
	}, nil
}

func (c *consensusService) Start(node NodeTransporter) (err error) {
	if c == nil {
		return errors.New("consensus: nil consensus service")
	}

	c.syncMx.Lock()
	defer c.syncMx.Unlock()

	nodeInfo := node.NodeInfo()

	c.raftID = raft.ServerID(nodeInfo.ID.String())

	raftConfig := raft.DefaultConfig()
	raftConfig.HeartbeatTimeout = time.Second * 5
	raftConfig.ElectionTimeout = raftConfig.HeartbeatTimeout
	raftConfig.LeaderLeaseTimeout = raftConfig.HeartbeatTimeout
	raftConfig.CommitTimeout = time.Second
	raftConfig.MaxAppendEntries = 128
	raftConfig.TrailingLogs = 256
	raftConfig.Logger = c.l
	raftConfig.LocalID = raft.ServerID(nodeInfo.ID.String())
	raftConfig.NoLegacyTelemetry = true
	raftConfig.SnapshotThreshold = 8192
	raftConfig.SnapshotInterval = 20 * time.Minute
	raftConfig.NoSnapshotRestoreOnStart = true

	if err := raft.ValidateConfig(raftConfig); err != nil {
		return err
	}

	c.transport, err = NewWarpnetConsensusTransport(
		node, newConsensusLogger(log.ErrorLevel.String(), "raft-libp2p-transport"),
	)
	if err != nil {
		log.Errorf("failed to create node transport: %v", err)
		return
	}
	log.Infoln("consensus: transport configured with local address:", c.transport.LocalAddr())

	hasState, err := raft.HasExistingState(c.logStore, c.stableStore, c.snapshotStore)
	if err != nil {
		return fmt.Errorf("consensus: failed to check existing state: %v", err)
	}

	isInitiator := raftConfig.LocalID == initiatorServerID

	if !hasState && isInitiator {
		log.Infoln("consensus: node is initiator - setting up new cluster")
		if err := c.bootstrap(raftConfig.LocalID); err != nil {
			return fmt.Errorf("consensus: setting up new cluster failed: %w", err)
		}
	}

	c.raft, err = raft.NewRaft(
		raftConfig,
		c.fsm,
		c.logStore,
		c.stableStore,
		c.snapshotStore,
		c.transport,
	)
	if err != nil {
		return fmt.Errorf("consensus: failed to create node: %w", err)
	}

	c.consensus.SetActor(libp2praft.NewActor(c.raft))
	err = c.retrier.Try(c.ctx, c.sync)
	if err != nil {
		return err
	}
	log.Infof("consensus: ready node %s with last index: %d", c.raftID, c.raft.LastIndex())
	c.node = node
	return nil
}

func (c *consensusService) bootstrap(id raft.ServerID) error {
	raftConf := raft.Configuration{}
	raftConf.Servers = append(raftConf.Servers, raft.Server{
		Suffrage: raft.Voter,
		ID:       id,
		Address:  raft.ServerAddress(id),
	})
	for _, info := range c.bootstrapNodes {
		if string(id) == info.ID.String() {
			continue
		}
		raftConf.Servers = append(raftConf.Servers, raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(info.ID.String()),
			Address:  raft.ServerAddress(info.ID.String()),
		})
	}

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

func (c *consensusService) waitClusterReady() error {
	clusterReadyChan := make(chan raft.ConfigurationFuture, 1)

	timeoutTimer := time.NewTimer(time.Second * 10)
	defer timeoutTimer.Stop()

	go func(crChan chan raft.ConfigurationFuture) {
		crChan <- c.raft.GetConfiguration()
	}(clusterReadyChan)

	select {
	case wait := <-clusterReadyChan:
		if wait.Error() != nil {
			return fmt.Errorf("consensus: config fetch error: %w", wait.Error())
		}
		log.Infof("consensus: cluster is ready: %v", wait.Configuration().Servers)
		break
	case <-timeoutTimer.C:
		return errors.New("consensus: getting configuration timeout — possibly broken cluster")
	}
	return nil
}

type consensusSync struct {
	ctx    context.Context
	raft   *raft.Raft
	raftID raft.ServerID
}

func (c *consensusService) sync() error {
	if c.raftID == "" {
		return errors.New("consensus: node id is not initialized")
	}

	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}

	if err := c.waitClusterReady(); err != nil {
		return err
	}

	leaderCtx, leaderCancel := context.WithTimeout(context.Background(), time.Minute)
	defer leaderCancel()

	cs := consensusSync{
		ctx:    c.ctx,
		raft:   c.raft,
		raftID: c.raftID,
	}

	log.Infoln("consensus: waiting for leader...")
	go cs.waitForLeader(leaderCtx)

	log.Infoln("consensus: waiting until we are promoted to a voter...")
	voterCtx, voterCancel := context.WithTimeout(context.Background(), time.Second*90)
	defer voterCancel()

	if err := cs.waitForVoter(voterCtx); err != nil {
		return fmt.Errorf("consensus: waiting to become a voter: %w", err)
	}
	log.Infoln("consensus: node received voter status")

	updatesCtx, updatesCancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer updatesCancel()

	if err := cs.waitForUpdates(updatesCtx); err != nil {
		log.Errorf("consensus: waiting for consensus updates: %v", err)
	}

	log.Infoln("consensus: sync complete")
	return nil
}

func (c *consensusSync) waitForLeader(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if c.ctx.Err() != nil {
			return
		}
		select {
		case <-ticker.C:
			addr, leaderID := c.raft.LeaderWithID()
			if addr == "" {
				continue
			}
			if c.raftID == leaderID {
				log.Infoln("consensus: node is a leader!")
				return
			}
			log.Infof("consensus: current leader: %s", leaderID)
			return

		case <-ctx.Done():
			log.Errorf("consensus: waiting for leader: %v", ctx.Err())
			return
		}
	}
}

func (c *consensusSync) waitForVoter(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	id := c.raftID
	for {
		if c.ctx.Err() != nil {
			return c.ctx.Err()
		}
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
		if c.ctx.Err() != nil {
			return c.ctx.Err()
		}
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
			if lastAppliedIndex > lastIndex {
				return errors.New("consensus: last applied index is greater than current index")
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
	c.waitSync()
	c.dropPrivateLeadership()

	if c.raft == nil {
		return
	}
	if info.ID.String() == "" {
		return
	}

	if _, leaderId := c.raft.LeaderWithID(); c.raftID != leaderId {
		return
	}

	id := raft.ServerID(info.ID.String())
	addr := raft.ServerAddress(info.ID.String())

	wait := c.raft.AddVoter(id, addr, 0, 30*time.Second)
	if wait.Error() != nil {
		log.Errorf("consensus: failed to add voted: %v", wait.Error())
		return
	}

	if _, err := c.cache.getVoter(id); errors.Is(err, errVoterNotFound) {
		log.Infof("consensus: new voter added %s", info.ID.String())
	}

	c.cache.addVoter(id, raft.Server{ // this cache only prevents voter removal flapping
		Suffrage: raft.Voter,
		ID:       id,
		Address:  addr,
	})
	return
}

func (c *consensusService) RemoveVoter(id warpnet.WarpPeerID) {
	c.waitSync()
	c.dropPrivateLeadership()

	if c.raft == nil {
		return
	}
	if id.String() == "" {
		return
	}

	if _, leaderId := c.raft.LeaderWithID(); c.raftID != leaderId {
		return
	}

	err := c.cache.removeVoter(raft.ServerID(id.String()))

	if errors.Is(err, errTooSoonToRemoveVoter) {
		log.Infof("consensus: removing voter %s is too soon, abort", id.String())
		return
	}
	log.Infof("consensus: removing voter %s", id.String())

	wait := c.raft.RemoveServer(raft.ServerID(id.String()), 0, 30*time.Second)
	if err := wait.Error(); err != nil {
		log.Errorf("consensus: failed to remove node: %s", wait.Error())
		return
	}
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
	if leaderId == "" {
		return errors.New("consensus: no leader found")
	}
	if leaderId == string(c.raftID) {
		_, err := c.CommitState(newState)
		if errors.Is(err, ErrNoRaftCluster) {
			return nil
		}
		log.Errorf("consensus: failed to add user validation to raft cluster: %v", err)
		return fmt.Errorf("consensus: failed to commit validate user state: %w", err)
	}

	resp, err := c.node.GenericStream(leaderId, event.PUBLIC_POST_NODE_VERIFY, newState)
	if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
		log.Errorf("consensus: failed to stream user validation to raft cluster: %v", err)
		return fmt.Errorf("consensus: node verify stream: %w", err)
	}
	if len(resp) == 0 {
		return errors.New("consensus: node verify stream returned empty response")
	}

	var errResp event.ErrorResponse
	if _ = json.JSON.Unmarshal(resp, &errResp); errResp.Message != "" {
		log.Errorf("consensus: verify response unmarshal failed: %v", errResp)
		return fmt.Errorf("consensus: verify response unmarshal failed: %w", errResp)
	}

	log.Infoln("consensus: user validated")
	return nil
}

func (c *consensusService) AskLeaderValidation() error {
	log.Infoln("consensus: asking for leader validation...")

	leaderId := c.LeaderID().String()
	if leaderId == "" {
		return errors.New("consensus: no leader found")
	}

	newState := map[string]string{
		"leader": leaderId,
	}

	if leaderId == string(c.raftID) {
		_, err := c.CommitState(newState)
		if errors.Is(err, ErrNoRaftCluster) {
			return nil
		}
		return fmt.Errorf("consensus: failed to commit validate leader state: %w", err)
	}

	resp, err := c.node.GenericStream(leaderId, event.PUBLIC_POST_NODE_VERIFY, newState)
	if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
		return fmt.Errorf("consensus: leader verify stream: %w", err)
	}
	if len(resp) == 0 {
		return errors.New("consensus: node leader verify stream returned empty response")
	}

	var errResp event.ErrorResponse
	if _ = json.JSON.Unmarshal(resp, &errResp); errResp.Message != "" {
		return fmt.Errorf("consensus: verify leader response unmarshal failed: %w", errResp)
	}

	log.Infoln("consensus: leader validated")
	return nil
}

func (c *consensusService) CommitState(newState KVState) (_ *KVState, err error) {
	c.waitSync()
	c.dropPrivateLeadership()

	if c.raft == nil {
		return nil, errors.New("consensus: nil node")
	}

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
	c.waitSync()

	if c.raft == nil {
		return nil, errors.New("consensus: nil node")
	}

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

func (c *consensusService) dropPrivateLeadership() {
	if c == nil || c.raft == nil {
		return
	}
	if !c.isPrivate {
		return
	}

	addr, leaderID := c.raft.LeaderWithID()
	if addr == "" {
		return
	}
	if c.raftID != leaderID {
		return
	}

	randomServer := c.bootstrapNodes[rand.Intn(len(c.bootstrapNodes))]

	log.Infof("consensus: dropping leadership because of private reachability %s, transferring to %s", leaderID, randomServer.ID)

	wait := c.raft.LeadershipTransferToServer(
		raft.ServerID(randomServer.ID.String()), raft.ServerAddress(randomServer.ID.String()),
	)
	if wait.Error() == nil {
		return
	}
	log.Errorf(
		"consensus: failed to send leader ship transfer to server %s: %v",
		randomServer.String(), wait.Error(),
	)
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
