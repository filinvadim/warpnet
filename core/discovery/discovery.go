package discovery

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"log"
	"strings"
)

// В libp2p, mDNS (Multicast DNS) — это механизм для локального обнаружения пиров (нод) в одной сети без использования централизованного сервера или предварительной настройки. Он позволяет нодам в одной локальной сети автоматически находить друг друга.
//  Как работает mDNS в libp2p
//
//    Broadcast запросы:
//    Нода отправляет multicast-запросы в локальную сеть на определённый адрес и порт, чтобы объявить своё присутствие и запросить присутствие других нод.
//
//    Ответы:
//    Ноды, которые получают запрос, отправляют свои адреса обратно в сеть. Таким образом, все ноды узнают друг о друге.
//
//    Обновление peerstore:
//    После обнаружения других нод их адреса автоматически добавляются в peerstore для дальнейшего использования.

type DiscoveryHandler func(warpnet.PeerAddrInfo)

type DiscoveryInfoStorer interface {
	ID() warpnet.WarpPeerID
	Peerstore() peerstore.Peerstore
	Network() warpnet.WarpNetwork
	Connect(p warpnet.PeerAddrInfo) error
	GenericStream(nodeId string, path stream.WarpRoute, data []byte) ([]byte, error)
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
	ctx      context.Context
	node     DiscoveryInfoStorer
	userRepo UserStorer
	nodeRepo NodeStorer

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
		ctx, nil, userRepo, nodeRepo,
		make(chan warpnet.PeerAddrInfo, 100), make(chan struct{}),
	}
}

func (s *discoveryService) Run(n DiscoveryInfoStorer) {
	if s == nil {
		return
	}
	log.Println("discovery service started")
	printPeers(n)
	defer log.Println("discovery service stopped")

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
			log.Println("discovery: print peers: recovered from panic:", r)
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
			log.Println("discovery: close recovered from panic:", r)
		}
	}()
	close(s.stopChan)
}

func (s *discoveryService) HandlePeerFound(pi warpnet.PeerAddrInfo) {
	if s == nil {
		return
	}
	if s.discoveryChan == nil {
		log.Println("discovery channel is nil")
		return
	}
	if len(s.discoveryChan) == cap(s.discoveryChan) {
		<-s.discoveryChan // drop old data
	}
	s.discoveryChan <- pi
}

func (s *discoveryService) handle(pi warpnet.PeerAddrInfo) {
	if s == nil || s.node == nil || s.userRepo == nil || s.nodeRepo == nil {
		log.Println("discovery service is not initialized")
		return
	}

	if pi.ID == "" {
		log.Printf("discovery: peer %s has no ID", pi.ID.String())
		return
	}
	if pi.ID == s.node.ID() {
		return
	}
	ok, err := s.nodeRepo.IsBlocklisted(s.ctx, pi.ID)
	if err != nil {
		log.Printf("discovery: failed to check blocklist: %s", err)
	}
	if ok {
		log.Printf("discovery: found blocklisted peer: %s", pi.ID.String())
		return
	}

	peerState := s.node.Network().Connectedness(pi.ID)
	if peerState == network.Connected || peerState == network.Limited {
		return
	}
	fmt.Printf("\033[1mdiscovery: found new peer: %s - %s \033[0m\n", pi.ID.String(), peerState)

	if err := s.node.Connect(pi); err != nil {
		log.Printf("discovery: failed to connect to new peer: %s, removing...", err)
		s.node.Peerstore().RemovePeer(pi.ID) // try add it again
		return
	}
	log.Printf("discovery: connected to new peer: %s", pi.ID)

	infoResp, err := s.node.GenericStream(pi.ID.String(), stream.InfoGetPublic, nil)
	if err != nil {
		log.Printf("discovery: failed to get info from new peer: %s", err)
		return
	}

	var info p2p.NodeInfo
	err = json.JSON.Unmarshal(infoResp, &info)
	if err != nil {
		log.Printf("discovery: failed to unmarshal info from new peer: %s", err)
		return
	}

	if err = s.nodeRepo.AddInfo(s.ctx, pi.ID, info); err != nil {
		log.Printf("discovery: failed to store info of new peer: %s", err)
	}

	bt, _ := json.JSON.Marshal(event.GetUserEvent{UserId: info.OwnerId})
	userResp, err := s.node.GenericStream(pi.ID.String(), stream.UserGetPublic, bt)
	if isBootstrapError(err) {
		return
	}
	if err != nil {
		log.Printf("discovery: failed to get info from new peer: %s", err)
		return
	}

	var user domain.User
	err = json.JSON.Unmarshal(userResp, &user)
	if err != nil {
		log.Printf("discovery: failed to unmarshal user from new peer: %s", err)
		return
	}
	_, err = s.userRepo.Create(user)
	if err != nil {
		log.Printf("discovery: failed to create user from new peer: %s", err)
	}
}

func isBootstrapError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "protocols not supported") {
		// bootstrap node doesn't support requesting
		return true
	}
	return false
}
