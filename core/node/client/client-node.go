package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"io"
	"log"
	"time"
)

const serverNodeAddrDefault = "/ip4/127.0.0.1/tcp/4001/p2p/"

type ClientStreamer interface {
	Send(peerAddr *warpnet.PeerAddrInfo, r warpnet.WarpRoute, data []byte) ([]byte, error)
}

type WarpClientNode struct {
	ctx      context.Context
	node     warpnet.P2PNode
	streamer ClientStreamer
	retrier  retrier.Retrier
}

func NewClientNode(ctx context.Context, clientInfo domain.AuthNodeInfo, conf config.Config) (_ *WarpClientNode, err error) {
	if clientInfo.Identity.Owner.NodeId == "" {
		return nil, errors.New("client node: server node ID is empty")
	}
	serverNodeId := clientInfo.Identity.Owner.NodeId

	client, err := libp2p.New(
		libp2p.NoListenAddrs,
		libp2p.DisableMetrics(),
		libp2p.DisableRelay(),
		libp2p.RandomIdentity,
		libp2p.Ping(false),
		libp2p.ForceReachabilityPrivate(),
		libp2p.DisableIdentifyAddressDiscovery(),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.PrivateNetwork(encrypting.ConvertToSHA256([]byte(conf.Node.PSK))),
		libp2p.UserAgent("warpnet-client"),
	)
	if err != nil {
		return nil, fmt.Errorf("client node: init %s", err)
	}
	serverAddr := serverNodeAddrDefault + serverNodeId
	maddr, err := warpnet.NewMultiaddr(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("client node: parsing server address: %s", err)
	}

	serverInfo, err := warpnet.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("client node: creating address info: %s", err)
	}

	client.Peerstore().AddAddrs(serverInfo.ID, serverInfo.Addrs, warpnet.PermanentAddrTTL)

	n := &WarpClientNode{
		ctx:     ctx,
		node:    client,
		retrier: retrier.New(time.Second * 5),
	}

	if len(client.Addrs()) != 0 {
		return nil, errors.New("client node must have no addresses")
	}

	n.streamer = stream.NewStreamPool(n.ctx, n.node)

	err = n.pairNodes(serverNodeId, clientInfo)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, err
	}
	log.Println("client-server nodes paired")
	log.Println("client node created:", n.node.ID())
	return n, nil
}

func (n *WarpClientNode) pairNodes(nodeId string, clientInfo domain.AuthNodeInfo) error {
	if n == nil {
		return errors.New("client node must not be nil")
	}
	bt, err := json.JSON.Marshal(clientInfo)
	if err != nil {
		return err
	}
	_, err = n.GenericStream(nodeId, stream.PairPrivate, bt)
	return err
}

func (n *WarpClientNode) GenericStream(nodeId string, path warpnet.WarpRoute, data []byte) ([]byte, error) {
	if n == nil {
		return nil, errors.New("client node must not be nil")
	}

	addrInfo, err := peer.AddrInfoFromString(serverNodeAddrDefault + nodeId)
	if err != nil {
		return nil, err
	}

	return n.streamer.Send(addrInfo, path, data)
}

func (n *WarpClientNode) Stop() {
	if n == nil {
		return
	}
	if err := n.node.Close(); err != nil {
		log.Println("client node stop fail:", err)
	}
}
