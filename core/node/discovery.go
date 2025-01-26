package node

import (
	"context"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"log"
	"sync"
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

type DiscoveryHandler func(peer.AddrInfo)

type DiscoveryInfoStorer interface {
	ID() string
	Peerstore() peerstore.Peerstore
	Node() host.Host
	StreamSend(peerID string, path types.WarpDiscriminator, data []byte) ([]byte, error)
}

type NodeStorer interface {
	GetOwner() (owner domain.Owner, err error)
	AddInfo(ctx context.Context, peerId types.WarpPeerID, info types.NodeInfo) error
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

	mx *sync.Mutex
}

func NewDiscoveryService(
	ctx context.Context,
	userRepo UserStorer,
	nodeRepo NodeStorer,
) *discoveryService {
	return &discoveryService{ctx, nil, userRepo, nodeRepo, new(sync.Mutex)}
}

func (s *discoveryService) JoinNode(n DiscoveryInfoStorer) {
	if s == nil {
		return
	}
	s.mx.Lock()
	defer s.mx.Unlock()
	s.node = n
}

func (s *discoveryService) HandlePeerFound(pi peer.AddrInfo) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s == nil || s.node == nil || s.userRepo == nil || s.nodeRepo == nil {
		log.Println("discovery service is not initialized")
		return
	}
	if pi.ID == "" {
		log.Printf("discovery: peer %s has no ID", pi.ID.String())
		return
	}
	if pi.ID.String() == s.node.ID() {
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
	log.Println("discovery: found new peer:", pi.ID.String())

	existedPeer := s.node.Peerstore().PeerInfo(pi.ID)
	if existedPeer.ID != "" {
		return
	}

	log.Printf("discovery: found new peer: %s %v", pi.ID.String(), pi.Addrs)

	if err := s.node.Node().Connect(context.Background(), pi); err != nil {
		log.Printf("discovery: failed to connect to new peer: %s", err)
		return
	}
	log.Printf("discovery: connected to new peer: %s", pi.ID)

	infoResp, err := s.node.StreamSend(pi.ID.String(), "/info/1.0.0", nil)
	if err != nil {
		log.Printf("discovery: failed to get info from new peer: %s", err)
		return
	}

	var info types.NodeInfo
	err = json.JSON.Unmarshal(infoResp, &info)
	if err != nil {
		log.Printf("discovery: failed to unmarshal info from new peer: %s", err)
		return
	}

	if err = s.nodeRepo.AddInfo(s.ctx, pi.ID, info); err != nil {
		log.Printf("discovery: failed to store info of new peer: %s", err)
	}

	bt, _ := json.JSON.Marshal(event.GetUserEvent{UserId: info.Owner.UserId})
	userResp, err := s.node.StreamSend(pi.ID.String(), "/user/1.0.0", bt)
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
