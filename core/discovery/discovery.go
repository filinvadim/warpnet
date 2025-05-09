package discovery

import (
	"context"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/retrier"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
	"time"
)

type DiscoveryHandler func(warpnet.PeerAddrInfo)

type DiscoveryInfoStorer interface {
	NodeInfo() warpnet.NodeInfo
	Peerstore() warpnet.WarpPeerstore
	Mux() warpnet.WarpProtocolSwitch
	Network() warpnet.WarpNetwork
	Connect(p warpnet.PeerAddrInfo) error
	GenericStream(nodeId string, path stream.WarpRoute, data any) ([]byte, error)
	AddOwnPublicAddress(remoteAddr string) error
}

type NodeStorer interface {
	BlocklistRemove(ctx context.Context, peerId warpnet.WarpPeerID) (err error)
	IsBlocklisted(ctx context.Context, peerId warpnet.WarpPeerID) (bool, error)
	Blocklist(ctx context.Context, peerId warpnet.WarpPeerID) error
}

type UserStorer interface {
	Create(user domain.User) (domain.User, error)
	Update(userId string, newUser domain.User) (domain.User, error)
	GetByNodeID(nodeID string) (user domain.User, err error)
}

type discoveryBoostrapNode struct {
	addrs        []warpnet.WarpAddress
	isDiscovered bool
}

type discoveryService struct {
	ctx      context.Context
	node     DiscoveryInfoStorer
	userRepo UserStorer
	nodeRepo NodeStorer
	version  *semver.Version

	handlers []DiscoveryHandler

	mx             *sync.RWMutex
	bootstrapAddrs map[warpnet.WarpPeerID]discoveryBoostrapNode

	retrier retrier.Retrier
	limiter *leakyBucketRateLimiter

	discoveryChan chan warpnet.PeerAddrInfo
	stopChan      chan struct{}
	syncDone      *atomic.Bool
}

//goland:noinspection ALL
func NewDiscoveryService(
	ctx context.Context,
	userRepo UserStorer,
	nodeRepo NodeStorer,
	handlers ...DiscoveryHandler,
) *discoveryService {
	addrs := make(map[warpnet.WarpPeerID]discoveryBoostrapNode)
	addrInfos, _ := config.ConfigFile.Node.AddrInfos()
	for _, info := range addrInfos {
		addrs[info.ID] = discoveryBoostrapNode{info.Addrs, false}
	}
	return &discoveryService{
		ctx, nil, userRepo, nodeRepo, config.ConfigFile.Version, handlers,
		new(sync.RWMutex), addrs, retrier.New(time.Second, 5, retrier.ArithmeticalBackoff),
		newRateLimiter(6, 1), make(chan warpnet.PeerAddrInfo, 100), make(chan struct{}),
		new(atomic.Bool),
	}
}

func NewBootstrapDiscoveryService(ctx context.Context, handlers ...DiscoveryHandler) *discoveryService {
	addrs := make(map[warpnet.WarpPeerID][]warpnet.WarpAddress)
	addrInfos, _ := config.ConfigFile.Node.AddrInfos()
	for _, info := range addrInfos {
		addrs[info.ID] = info.Addrs
	}
	return NewDiscoveryService(ctx, nil, nil, handlers...)
}

func (s *discoveryService) Run(n DiscoveryInfoStorer) error {
	if s == nil {
		return errors.New("nil discovery service")
	}
	if s.discoveryChan == nil {
		return errors.New("discovery channel is nil")
	}
	log.Infoln("discovery: service started")

	s.node = n

	go func() {
		for {
			select {
			case <-s.ctx.Done():
				log.Errorf("discovery: context closed")
				return
			case <-s.stopChan:
				return
			case info, ok := <-s.discoveryChan:
				if !ok {
					log.Infoln("discovery: service closed")
					return
				}
				s.handle(info)
			}
		}
	}()
	return s.syncBootstrapDiscovery()
}

func (s *discoveryService) syncBootstrapDiscovery() error {
	defer func() {
		s.syncDone.Store(true)
	}()

	s.mx.RLock()
	for id, discNode := range s.bootstrapAddrs {
		if s.node.NodeInfo().ID == id {
			continue
		}
		s.discoveryChan <- warpnet.PeerAddrInfo{ID: id, Addrs: discNode.addrs}
	}
	s.mx.RUnlock()

	if s.node.NodeInfo().IsBootstrap() {
		return nil
	}

	tryouts := 30
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		var isAllDiscovered = true

		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-s.stopChan:
			return nil
		case <-ticker.C:
			s.mx.RLock()
			for _, discNode := range s.bootstrapAddrs {
				if !discNode.isDiscovered {
					isAllDiscovered = false
					break
				}
			}
			s.mx.RUnlock()
			if isAllDiscovered {
				log.Infof("discovery: all bootstrap addresses discovered")
				return nil
			}

			tryouts--
			if tryouts == 0 {
				return errors.New("discovery: all discovery attempts failed")
			}
		}
	}
}

func (s *discoveryService) Close() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("discovery: close recovered from panic: %v", r)
		}
	}()
	if s.stopChan == nil {
		return
	}
	close(s.stopChan)
	close(s.discoveryChan)
}

func (s *discoveryService) DefaultDiscoveryHandler(peerInfo warpnet.PeerAddrInfo) {
	if s == nil {
		return
	}
	defer func() { recover() }()

	if peerInfo.ID == s.node.NodeInfo().ID {
		return
	}

	if err := s.node.Connect(peerInfo); err != nil {
		log.Errorf(
			"discovery: default handler: failed to connect to peer %s: %v",
			peerInfo.String(),
			err,
		)
		return
	}
	s.node.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour*8)

	for _, h := range s.handlers {
		h(peerInfo)
	}
	return
}

func (s *discoveryService) HandlePeerFound(pi warpnet.PeerAddrInfo) {
	if s == nil {
		return
	}
	defer func() { recover() }()

	if !s.limiter.Allow() {
		return
	}

	if len(s.discoveryChan) == cap(s.discoveryChan) {
		log.Warnf("discovery: channel overflow %d", cap(s.discoveryChan))
		<-s.discoveryChan // drop old data
	}
	s.discoveryChan <- pi
}

func (s *discoveryService) handle(pi warpnet.PeerAddrInfo) {
	if s == nil || s.node == nil || s.nodeRepo == nil || s.userRepo == nil {
		return
	}

	if pi.ID == "" || len(pi.Addrs) == 0 {
		return
	}

	if pi.ID == s.node.NodeInfo().ID {
		return
	}

	ok, err := s.nodeRepo.IsBlocklisted(s.ctx, pi.ID)
	if err != nil {
		log.Errorf("discovery: failed to check blocklist: %s", err)
	}
	if ok {
		log.Infof("discovery: found blocklisted peer: %s", pi.ID.String())
		return
	}

	for _, h := range s.handlers {
		h(pi)
	}

	if s.isMineBootstrapNodes(pi) {
		return
	}

	if !s.hasPublicAddresses(pi.Addrs) {
		log.Warnf("discovery: peer %s has no public addresses: %v", pi.ID.String(), pi.Addrs)
		return
	}

	if err := s.node.Connect(pi); err != nil {
		log.Errorf("discovery: failed to connect to new peer: %s...", err)
		return
	}

	info, err := s.requestNodeInfo(pi)
	if err != nil {
		log.Errorf("discovery: %v", err)
		return
	}
	if err := s.node.AddOwnPublicAddress(info.RequesterAddr); err != nil {
		log.Errorf("discovery: failed to add own public address: %s %v", info.RequesterAddr, err)
	}

	if info.IsBootstrap() {
		s.markBootstrapDiscovered(pi)
		return
	}

	existedUser, err := s.userRepo.GetByNodeID(pi.ID.String())
	if !errors.Is(err, database.ErrUserNotFound) && !existedUser.IsOffline {
		return
	}

	fmt.Printf("\033[1mdiscovery: connected to new peer: %s \033[0m\n", pi.String())

	user, err := s.requestNodeUser(pi, info.OwnerId)
	if err != nil {
		log.Errorf("discovery: %v", err)
		return
	}

	newUser, err := s.userRepo.Create(user)
	if errors.Is(err, database.ErrUserAlreadyExists) {
		newUser, _ = s.userRepo.Update(user.Id, user)
		return
	}
	if err != nil {
		log.Errorf("discovery: failed to create user from new peer: %s", err)
		return
	}
	log.Infof(
		"discovery: new user added: id: %s, name: %s, node_id: %s, created_at: %s, latency: %d",
		newUser.Id,
		newUser.Username,
		newUser.NodeId,
		newUser.CreatedAt,
		newUser.Latency,
	)
}

func (s *discoveryService) isMineBootstrapNodes(pi warpnet.PeerAddrInfo) bool {
	if !s.syncDone.Load() {
		return false
	}

	s.mx.Lock()
	defer s.mx.Unlock()

	_, ok := s.bootstrapAddrs[pi.ID]
	return ok
}

func (s *discoveryService) markBootstrapDiscovered(pi warpnet.PeerAddrInfo) {
	s.mx.Lock()
	defer s.mx.Unlock()

	bNode, ok := s.bootstrapAddrs[pi.ID]
	if !ok {
		return
	}
	s.node.Peerstore().AddAddrs(pi.ID, bNode.addrs, time.Hour*24) // update local bootstrap addresses with public ones
	bNode.isDiscovered = true
	s.bootstrapAddrs[pi.ID] = bNode
}

func (s *discoveryService) requestNodeInfo(pi warpnet.PeerAddrInfo) (info warpnet.NodeInfo, err error) {
	if s == nil {
		return info, err
	}

	newNodeInfo := s.node.Peerstore().PeerInfo(pi.ID)
	log.Infof("discovery: requesting node info for peer %s", newNodeInfo.String())

	infoResp, err := s.node.GenericStream(pi.ID.String(), event.PUBLIC_GET_INFO, nil)
	if err != nil {
		return info, fmt.Errorf("failed to get info from new peer %s: %v", pi.ID.String(), err)
	}

	if infoResp == nil || len(infoResp) == 0 {
		return info, fmt.Errorf("no info response from new peer %s", pi.ID.String())
	}

	err = json.JSON.Unmarshal(infoResp, &info)
	if err != nil {
		return info, fmt.Errorf("failed to unmarshal info from new peer: %s %v", infoResp, err)
	}
	if info.OwnerId == "" {
		return info, fmt.Errorf("node info %s has no owner", pi.ID.String())
	}
	return info, nil
}

func (s *discoveryService) requestNodeUser(pi warpnet.PeerAddrInfo, userId string) (user domain.User, err error) {
	if s == nil {
		return user, err
	}

	getUserEvent := event.GetUserEvent{UserId: userId}

	userResp, err := s.node.GenericStream(pi.ID.String(), event.PUBLIC_GET_USER, getUserEvent)
	if err != nil {
		return user, fmt.Errorf("failed to user data from new peer %s: %v", pi.ID.String(), err)
	}

	if userResp == nil || len(userResp) == 0 {
		return user, fmt.Errorf("no user response from new peer %s", pi.String())
	}

	err = json.JSON.Unmarshal(userResp, &user)
	if err != nil {
		return user, fmt.Errorf("failed to unmarshal user from new peer: %v", err)
	}

	user.IsOffline = false
	user.NodeId = pi.ID.String()
	user.Latency = int64(s.node.Peerstore().LatencyEWMA(pi.ID))
	return user, nil
}

func (s *discoveryService) hasPublicAddresses(addrs []warpnet.WarpAddress) bool {
	for _, addr := range addrs {
		if warpnet.IsPublicMultiAddress(addr) {
			return true
		}
	}
	return false
}
