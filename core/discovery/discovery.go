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
	"github.com/libp2p/go-libp2p/core/peer"
	log "github.com/sirupsen/logrus"
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
	BlocklistRemove(ctx context.Context, peerId peer.ID) (err error)
	IsBlocklisted(ctx context.Context, peerId peer.ID) (bool, error)
	Blocklist(ctx context.Context, peerId peer.ID) error
}

type UserStorer interface {
	Create(user domain.User) (domain.User, error)
	Update(userId string, newUser domain.User) (domain.User, error)
	GetByNodeID(nodeID string) (user domain.User, err error)
}

type discoveryService struct {
	ctx            context.Context
	node           DiscoveryInfoStorer
	userRepo       UserStorer
	nodeRepo       NodeStorer
	version        *semver.Version
	bootstrapAddrs map[warpnet.WarpPeerID][]warpnet.WarpAddress
	retrier        retrier.Retrier
	limiter        *leakyBucketRateLimiter

	discoveryChan chan warpnet.PeerAddrInfo
	stopChan      chan struct{}
}

//goland:noinspection ALL
func NewDiscoveryService(
	ctx context.Context,
	userRepo UserStorer,
	nodeRepo NodeStorer,
) *discoveryService {
	addrs := make(map[warpnet.WarpPeerID][]warpnet.WarpAddress)
	addrInfos, _ := config.ConfigFile.Node.AddrInfos()
	for _, info := range addrInfos {
		addrs[info.ID] = info.Addrs
	}
	return &discoveryService{
		ctx, nil, userRepo, nodeRepo, config.ConfigFile.Version,
		addrs, retrier.New(time.Second, 5, retrier.ArithmeticalBackoff),
		newRateLimiter(6, 1), make(chan warpnet.PeerAddrInfo, 100), make(chan struct{}),
	}
}

func NewBootstrapDiscoveryService(ctx context.Context) *discoveryService {
	addrs := make(map[warpnet.WarpPeerID][]warpnet.WarpAddress)
	addrInfos, _ := config.ConfigFile.Node.AddrInfos()
	for _, info := range addrInfos {
		addrs[info.ID] = info.Addrs
	}
	return NewDiscoveryService(ctx, nil, nil)
}

func (s *discoveryService) Run(n DiscoveryInfoStorer) {
	if s == nil {
		return
	}
	log.Infoln("discovery: service started")

	s.node = n

	err := s.retrier.Try(s.ctx, func() error {
		return s.getPublicAddress()
	})
	if err != nil {
		log.Infof("discovery: %v", err)
		return
	}

	if s.discoveryChan == nil {
		return
	}

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
}

func (s *discoveryService) getPublicAddress() error {
	isAddrAdded := false
	for id, addrs := range s.bootstrapAddrs {
		info, err := s.requestNodeInfo(warpnet.PeerAddrInfo{ID: id, Addrs: addrs})
		if err != nil {
			log.Errorf("discovery: initial request node info %v", err)
			continue
		}
		if err := s.node.AddOwnPublicAddress(info.RequesterAddr); err != nil {
			log.Errorf("discovery: initial adding own public address: %s %v", info.RequesterAddr, err)
			continue
		}
		isAddrAdded = true
	}
	if !isAddrAdded {
		return errors.New("discovery: no public address found")
	}
	return nil
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
	for len(s.discoveryChan) > 0 {
		<-s.discoveryChan
	}
	close(s.discoveryChan)
}

func (s *discoveryService) DefaultDiscoveryHandler(peerInfo warpnet.PeerAddrInfo) {
	if s == nil {
		return
	}
	defer func() { recover() }()

	if !s.limiter.Allow() {
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
	log.Debugf("discovery: default handler: connected to peer: %s %s", peerInfo.Addrs, peerInfo.ID)
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

	if pi.ID == "" {
		log.Errorf("discovery: peer %s has no ID", pi.ID.String())
		return
	}

	log.Debugf("discovery: handling peer %s %v", pi.ID.String(), pi.Addrs)

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

	if !s.hasPublicAddresses(pi.Addrs) {
		log.Infof("discovery: peer %s has no public addresses", pi.ID.String())
		return
	}

	bAddrs, ok := s.bootstrapAddrs[pi.ID]
	if ok {
		// update local bootstrap addresses with public ones
		pi.Addrs = append(pi.Addrs, bAddrs...)
	}
	s.node.Peerstore().AddAddrs(pi.ID, pi.Addrs, time.Hour)

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

	if info.OwnerId == warpnet.BootstrapOwner { // bootstrap node
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

func (s *discoveryService) requestNodeInfo(pi warpnet.PeerAddrInfo) (info warpnet.NodeInfo, err error) {
	if s == nil {
		return info, err
	}

	infoResp, err := s.node.GenericStream(pi.ID.String(), event.PUBLIC_GET_INFO, nil)
	if err != nil {
		return info, fmt.Errorf("failed to get info from new peer %s: %v", pi.String(), err)
	}

	if infoResp == nil || len(infoResp) == 0 {
		return info, fmt.Errorf("no info response from new peer %s", pi.String())
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
