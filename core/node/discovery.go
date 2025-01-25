package node

import (
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/core/types"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"log"
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

type DiscoveryInfoStorer interface {
	ID() string
	Peerstore() peerstore.Peerstore
	Node() host.Host
	StreamSend(peerID string, path types.WarpDiscriminator, data []byte) ([]byte, error)
}

type bootstrapDiscoveryHandler struct {
	node DiscoveryInfoStorer
}

func NewBootstrapDiscovery(h DiscoveryInfoStorer) *bootstrapDiscoveryHandler {
	return &bootstrapDiscoveryHandler{h}
}

func (n *bootstrapDiscoveryHandler) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("found new peer: %s %v", pi.ID.String(), pi.Addrs)
	if err := n.node.Node().Connect(context.Background(), pi); err != nil {
		log.Printf("failed to connect to new peer: %s", err)
		return
	}
	log.Printf("connected to new peer: %s", pi.ID)
}

type NodeStorer interface {
	GetOwner() (owner domain.Owner, err error)
	AddInfo(ctx context.Context, peerId types.WarpPeerID, info types.NodeInfo) error
	RemoveInfo(ctx context.Context, peerId peer.ID) (err error)
	BlocklistRemove(ctx context.Context, peerId peer.ID) (err error)
	IsBlocklisted(ctx context.Context, peerId peer.ID) (bool, error)
	Blocklist(ctx context.Context, peerId peer.ID) error
}

type memberDiscoveryHandler struct {
	node     DiscoveryInfoStorer
	userRepo *database.UserRepo
	nodeRepo NodeStorer
}

func NewMemberDiscovery(
	n DiscoveryInfoStorer,
	userRepo *database.UserRepo,
	nodeRepo NodeStorer,
) *memberDiscoveryHandler {
	return &memberDiscoveryHandler{n, userRepo, nodeRepo}
}

func (n *memberDiscoveryHandler) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Println("handle peer found")
	ctx := context.Background() // TODO
	if pi.ID == "" {
		log.Printf("discovery: peer %s has no ID", pi.ID.String())
		return
	}
	if pi.ID.String() == n.node.ID() {
		return
	}
	ok, err := n.nodeRepo.IsBlocklisted(ctx, pi.ID)
	if err != nil {
		log.Printf("discovery: failed to check blocklist: %s", err)
	}
	if ok {
		log.Printf("discovery: found blocklisted peer: %s", pi.ID.String())
		return
	}

	existedPeer := n.node.Peerstore().PeerInfo(pi.ID)
	if existedPeer.ID != "" {
		log.Printf("discovery: found existing peer: %s, skipping...", existedPeer.ID)
	}

	log.Printf("discovery: found new peer: %s %v", pi.ID.String(), pi.Addrs)

	if err := n.node.Node().Connect(context.Background(), pi); err != nil {
		log.Printf("discovery: failed to connect to new peer: %s", err)
		return
	}
	log.Printf("discovery: connected to new peer: %s", pi.ID)

	infoResp, err := n.node.StreamSend(pi.ID.String(), "/info/1.0.0", nil)
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

	if err = n.nodeRepo.AddInfo(ctx, pi.ID, info); err != nil {
		log.Printf("discovery: failed to store info of new peer: %s", err)
	}

	bt, _ := json.JSON.Marshal(event.GetUserEvent{info.Owner.UserId})
	userResp, err := n.node.StreamSend(pi.ID.String(), "/user/1.0.0", bt)
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
	_, err = n.userRepo.Create(user)
	if err != nil {
		log.Printf("discovery: failed to create user from new peer: %s", err)
	}
}
