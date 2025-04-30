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
	"github.com/libp2p/go-libp2p/core/network"
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
		addrs, make(chan warpnet.PeerAddrInfo, 100), make(chan struct{}),
	}
}

func NewBootstrapDiscoveryService(ctx context.Context) *discoveryService {
	addrs := make(map[warpnet.WarpPeerID][]warpnet.WarpAddress)
	addrInfos, _ := config.ConfigFile.Node.AddrInfos()
	for _, info := range addrInfos {
		addrs[info.ID] = info.Addrs
	}
	return &discoveryService{
		ctx:            ctx,
		version:        config.ConfigFile.Version,
		bootstrapAddrs: addrs,
	}
}

func (s *discoveryService) Run(n DiscoveryInfoStorer) {
	if s == nil {
		return
	}
	log.Infoln("discovery: service started")
	printPeers(n)

	s.node = n

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

func printPeers(n DiscoveryInfoStorer) {
	defer func() {
		if r := recover(); r != nil { // could panic
			log.Errorf("discovery: print peers: recovered from panic: %v", r)
		}
	}()
	for _, id := range n.Peerstore().Peers() {
		if n.NodeInfo().ID == id {
			continue
		}

		info := n.Peerstore().PeerInfo(id)

		fmt.Printf("\033[1mdiscovery: known peer: %s \033[0m\n", info.String())
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
	for len(s.discoveryChan) > 0 {
		<-s.discoveryChan
	}
	close(s.discoveryChan)
}

func (s *discoveryService) DefaultDiscoveryHandler(peerInfo warpnet.PeerAddrInfo) {
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

	if len(s.discoveryChan) == cap(s.discoveryChan) {
		log.Errorf("discovery: channel overflow %d", cap(s.discoveryChan))
		<-s.discoveryChan // drop old data
	}
	s.discoveryChan <- pi
}

func (s *discoveryService) handle(pi warpnet.PeerAddrInfo) {
	if s == nil || s.node == nil || s.userRepo == nil || s.nodeRepo == nil {
		return
	}

	if pi.ID == "" {
		log.Errorf("discovery: peer %s has no ID", pi.ID.String())
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

	peerState := s.node.Network().Connectedness(pi.ID)

	isConnected := peerState == network.Connected || peerState == network.Limited

	bAddrs, ok := s.bootstrapAddrs[pi.ID]
	if ok {
		// update local bootstrap addresses with public ones
		pi.Addrs = append(pi.Addrs, bAddrs...)
		s.node.Peerstore().AddAddrs(pi.ID, pi.Addrs, time.Hour*24)
	}

	if !isConnected {
		fmt.Printf("\033[1mdiscovery: found new peer: %s - %s \033[0m\n", pi.String(), peerState)

		if err := s.node.Connect(pi); err != nil {
			log.Errorf("discovery: failed to connect to new peer: %s...", err)
			return
		}
		log.Infof("discovery: connected to new peer: %s", pi.ID)
	}

	if s.isBootstrapNode(pi) {
		return
	}

	existedUser, err := s.userRepo.GetByNodeID(pi.ID.String())
	if !errors.Is(err, database.ErrUserNotFound) && !existedUser.IsOffline {
		return
	}

	infoResp, err := s.node.GenericStream(pi.ID.String(), event.PUBLIC_GET_INFO, nil)
	if err != nil {
		log.Errorf("discovery: failed to get info from new peer: %s", err)
		return
	}

	var info warpnet.NodeInfo
	err = json.JSON.Unmarshal(infoResp, &info)
	if err != nil {
		log.Errorf("discovery: failed to unmarshal info from new peer: %s", err)
		return
	}

	if info.OwnerId == "" {
		log.Infof("discovery: peer %s has no owner", pi.ID.String())
		return
	}

	getUserEvent := event.GetUserEvent{UserId: info.OwnerId}

	userResp, err := s.node.GenericStream(pi.ID.String(), event.PUBLIC_GET_USER, getUserEvent)
	if err != nil {
		log.Errorf("discovery: failed to get info from new peer: %s", err)
		return
	}
	latency := s.node.Peerstore().LatencyEWMA(pi.ID)

	var user domain.User
	err = json.JSON.Unmarshal(userResp, &user)
	if err != nil {
		log.Errorf("discovery: failed to unmarshal user from new peer: %s", err)
		return
	}

	user.IsOffline = false
	user.NodeId = pi.ID.String()
	user.Latency = int64(latency)
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
		"discovery: new user added: id %s, name %s, node_id %s, created_at %s, latency: %d",
		newUser.Id,
		newUser.Username,
		newUser.NodeId,
		newUser.CreatedAt,
		newUser.Latency,
	)
}

func (s *discoveryService) isBootstrapNode(pi warpnet.PeerAddrInfo) bool {
	otherProtocols, err := s.node.Peerstore().GetProtocols(pi.ID)
	if err != nil {
		log.Errorf("discovery: failed to get new node protocols: %s", err)
		return false
	}
	mineProtocols := s.node.Mux().Protocols()
	if len(otherProtocols) < len(mineProtocols) {
		// bootstrap node supports only routing protocols
		return true
	}
	return false
}
