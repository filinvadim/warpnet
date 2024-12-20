package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	client "github.com/filinvadim/warpnet/node-client"
	"github.com/filinvadim/warpnet/node/server"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
	"log"
	"os"
)

type NodeServer interface {
	Start() error
	Stop() error
}

type NodeService struct {
	ctx    context.Context
	server NodeServer
	client *client.NodeClient

	nodeRepo *database.NodeRepo
	authRepo *database.AuthRepo
	userRepo *database.UserRepo

	stopChan chan struct{}
}

func NewNodeService(
	ctx context.Context,
	ownIP string,
	predefinedHosts []string,
	db *storage.DB,
	interrupt chan os.Signal,
) (*NodeService, error) {

	nodeRepo := database.NewNodeRepo(db)
	authRepo := database.NewAuthRepo(db)
	followRepo := database.NewFollowRepo(db)
	timelineRepo := database.NewTimelineRepo(db)
	tweetRepo := database.NewTweetRepo(db)
	userRepo := database.NewUserRepo(db)
	replyRepo := database.NewRepliesRepo(db)

	cli, err := client.NewNodeClient(ctx)
	if err != nil {
		return nil, err
	}
	handler, err := server.NewNodeHandler(
		ownIP,
		nodeRepo,
		authRepo,
		userRepo,
		tweetRepo,
		timelineRepo,
		followRepo,
		replyRepo,
		cli,
		interrupt,
	)
	if err != nil {
		return nil, fmt.Errorf("node handler: %w", err)
	}
	srv, err := server.NewNodeServer(ctx, handler)
	if err != nil {
		return nil, fmt.Errorf("node server: %w", err)
	}

	return &NodeService{
		ctx, srv, cli, nodeRepo,
		authRepo, userRepo, make(chan struct{}),
	}, nil
}

func (ds *NodeService) Run() {
	if err := ds.server.Start(); err != nil {
		log.Fatalf("node server: %v", err)
	}
}

func (ds *NodeService) Stop() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	close(ds.stopChan)
	if err := ds.server.Stop(); err != nil {
		log.Println(err)
	}
}

func NewP2PNode(ctx context.Context, nodeRepo *database.NodeRepo) host.Host {
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		log.Fatalln(err)
	}
	rawKey, err := key.Raw()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(rawKey))

	store, err := pstoreds.NewPeerstore(ctx, nodeRepo, pstoreds.DefaultOpts())
	if err != nil {
		log.Fatalln(err)
	}
	node, err := libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/16969",
			"/ip4/0.0.0.0/udp/16969/quic-v1",
			"/ip4/0.0.0.0/udp/16969/quic-v1/webtransport",
			"/ip6/::/tcp/16969",
			"/ip6/::/udp/16969/quic-v1",
			"/ip6/::/udp/16969/quic-v1/webtransport",
		),
		libp2p.Identity(key),
		libp2p.Ping(true),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoNATv2(),
		libp2p.NATPortMap(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.PrivateNetwork([]byte("test")),
		libp2p.UserAgent("myUserAgent"),
		libp2p.EnableHolePunching(),
		libp2p.Peerstore(store),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			idht, err := dht.New(ctx, h)
			if err != nil {
				return nil, err
			}
			go idht.Bootstrap(ctx)
			return idht, nil
		}),
	)
	if err != nil {
		panic(err)
	}

	mdnsService := mdns.NewMdnsService(node, "warpnet", &discoveryNotifee{node})
	defer mdnsService.Close()

	bootstrapNodes := []string{}
	for _, addr := range bootstrapNodes {
		maddr, _ := multiaddr.NewMultiaddr(addr)
		peerInfo, _ := peer.AddrInfoFromP2pAddr(maddr)
		if err := node.Connect(ctx, *peerInfo); err != nil {
			log.Printf("Failed to connect to bootstrap node: %s", err)
		} else {
			log.Printf("Connected to bootstrap node: %s", peerInfo.ID)
		}
	}

	if err := node.Close(); err != nil {
		panic(err)
	}
	return node
}

type discoveryNotifee struct {
	host host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	log.Printf("Found new peer: %s", pi.ID.String())
	if err := n.host.Connect(context.Background(), pi); err != nil {
		log.Printf("Failed to connect to new peer: %s", err)
	} else {
		log.Printf("Connected to new peer: %s", pi.ID)
	}
}
