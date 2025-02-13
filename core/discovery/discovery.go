package discovery

import (
	"context"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/p2p"
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
	ID() warpnet.WarpPeerID
	Peerstore() warpnet.WarpPeerstore
	Mux() warpnet.WarpProtocolSwitch
	Network() warpnet.WarpNetwork
	Connect(p warpnet.PeerAddrInfo) error
	GenericStream(nodeId string, path stream.WarpRoute, data any) ([]byte, error)
}

type NodeStorer interface {
	AddInfo(ctx context.Context, peerId warpnet.WarpPeerID, info p2p.NodeInfo) error
	RemoveInfo(ctx context.Context, peerId peer.ID) (err error)
	BlocklistRemove(ctx context.Context, peerId peer.ID) (err error)
	IsBlocklisted(ctx context.Context, peerId peer.ID) (bool, error)
	Blocklist(ctx context.Context, peerId peer.ID) error
}

type UserStorer interface {
	Create(user domain.User) (domain.User, error)
}

type discoveryService struct {
	ctx           context.Context
	node          DiscoveryInfoStorer
	userRepo      UserStorer
	nodeRepo      NodeStorer
	version       *semver.Version
	discoveryChan chan warpnet.PeerAddrInfo
	stopChan      chan struct{}
}

//goland:noinspection ALL
func NewDiscoveryService(
	ctx context.Context,
	userRepo UserStorer,
	nodeRepo NodeStorer,
) *discoveryService {
	return &discoveryService{
		ctx, nil, userRepo, nodeRepo, config.ConfigFile.Version,
		make(chan warpnet.PeerAddrInfo, 100), make(chan struct{}),
	}
}

func (s *discoveryService) Run(n DiscoveryInfoStorer) {
	if s == nil {
		return
	}
	log.Infoln("discovery service started")
	printPeers(n)
	defer log.Infoln("discovery service stopped")

	s.node = n

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.stopChan:
		case info, ok := <-s.discoveryChan:
			if !ok {
				return
			}
			s.handle(info)
		}
	}
}

func printPeers(n DiscoveryInfoStorer) {
	defer func() {
		if r := recover(); r != nil { // could panic
			log.Errorf("discovery: print peers: recovered from panic:", r)
		}
	}()
	for _, id := range n.Peerstore().Peers() {
		if n.ID() == id {
			continue
		}
		fmt.Printf("\033[1mknown peer: %s \033[0m\n", id.String())
	}
}

func (s *discoveryService) Close() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("discovery: close recovered from panic:", r)
		}
	}()
	close(s.stopChan)
	for len(s.discoveryChan) > 0 {
		<-s.discoveryChan
	}
	close(s.discoveryChan)
}

func (s *discoveryService) HandlePeerFound(pi warpnet.PeerAddrInfo) {
	if s == nil {
		return
	}
	if s.discoveryChan == nil {
		log.Errorf("discovery channel is nil")
		return
	}
	if len(s.discoveryChan) == cap(s.discoveryChan) {
		<-s.discoveryChan // drop old data
	}
	s.discoveryChan <- pi
}

func (s *discoveryService) handle(pi warpnet.PeerAddrInfo) {
	if s == nil || s.node == nil || s.userRepo == nil || s.nodeRepo == nil {
		log.Errorf("discovery service is not initialized")
		return
	}

	if pi.ID == "" {
		log.Errorf("discovery: peer %s has no ID", pi.ID.String())
		return
	}
	if pi.ID == s.node.ID() {
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
	if peerState == network.Connected || peerState == network.Limited {
		return
	}
	fmt.Printf("\033[1mdiscovery: found new peer: %s - %s \033[0m\n", pi.ID.String(), peerState)

	if err := s.node.Connect(pi); err != nil {
		log.Errorf("discovery: failed to connect to new peer: %s, removing...", err)
		return
	}
	log.Infof("discovery: connected to new peer: %s", pi.ID)

	if s.isBootstrapNode(pi) {
		log.Warningln("discovery: bootstrap node doesn't support requesting", pi.ID.String())
		return
	}

	infoResp, err := s.node.GenericStream(pi.ID.String(), event.PUBLIC_GET_INFO, nil)
	if err != nil {
		log.Errorf("discovery: failed to get info from new peer: %s", err)
		return
	}

	var info p2p.NodeInfo
	err = json.JSON.Unmarshal(infoResp, &info)
	if err != nil {
		log.Errorf("discovery: failed to unmarshal info from new peer: %s", err)
		return
	}

	if info.Version.Major() < s.version.Major() {
		log.Infof("discovery: peer %s has old version %s", pi.ID.String(), info.Version.String())
		return
	}

	if err = s.nodeRepo.AddInfo(s.ctx, pi.ID, info); err != nil {
		log.Errorf("discovery: failed to store info of new peer: %s", err)
	}

	if info.OwnerId == "" {
		log.Infof("discovery: peer %s has no owner", pi.ID.String())
		return
	}

	getUserEvent := event.GetUserEvent{UserId: info.OwnerId}
	now := time.Now()
	userResp, err := s.node.GenericStream(pi.ID.String(), event.PUBLIC_GET_USER, getUserEvent)
	if err != nil {
		log.Errorf("discovery: failed to get info from new peer: %s", err)
		return
	}
	rtt := time.Since(now)

	var user domain.User
	err = json.JSON.Unmarshal(userResp, &user)
	if err != nil {
		log.Errorf("discovery: failed to unmarshal user from new peer: %s", err)
		return
	}
	user.Rtt = int64(rtt)
	newUser, err := s.userRepo.Create(user)
	if errors.Is(err, database.ErrUserAlreadyExists) {
		return
	}
	if err != nil {
		log.Errorf("discovery: failed to create user from new peer: %s", err)
		return
	}
	bt, _ := json.JSON.MarshalIndent(newUser, "", "  ")
	log.Infoln("new user added:", string(bt))
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
		log.Infof("protocols num comparison: %d < %d", len(mineProtocols), len(otherProtocols))
		return true
	}
	return false
}
