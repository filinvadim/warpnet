package client

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/filinvadim/warpnet/security"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	log "github.com/sirupsen/logrus"
	"io"
	"sync/atomic"
	"time"
)

type ClientStreamer interface {
	Send(peerAddr warpnet.PeerAddrInfo, r stream.WarpRoute, data []byte) ([]byte, error)
}

type WarpClientNode struct {
	ctx            context.Context
	clientNode     warpnet.P2PNode
	streamer       ClientStreamer
	retrier        retrier.Retrier
	serverNodeAddr string
	privKey        warpnet.WarpPrivateKey
	psk            security.PSK
	isRunning      *atomic.Bool
}

func NewClientNode(ctx context.Context, psk security.PSK) (_ *WarpClientNode, err error) {
	privKey, err := security.GenerateKeyFromSeed([]byte(rand.Text()))
	if err != nil {
		log.Fatalf("fail generating key: %v", err)
	}
	serverNodeAddrDefault := fmt.Sprintf("/ip4/127.0.0.1/tcp/%s/p2p/", config.ConfigFile.Node.Port)

	n := &WarpClientNode{
		ctx:            ctx,
		clientNode:     nil,
		retrier:        retrier.New(time.Second*5, 10, retrier.ExponentialBackoff),
		serverNodeAddr: serverNodeAddrDefault,
		privKey:        privKey.(warpnet.WarpPrivateKey),
		isRunning:      new(atomic.Bool),
		psk:            psk,
	}

	return n, nil
}

func (n *WarpClientNode) Pair(serverInfo domain.AuthNodeInfo) error {
	if n == nil {
		return errors.New("client node not initialized")
	}
	if serverInfo.NodeInfo.ID.String() == "" {
		return errors.New("client node: server node ID is empty")
	}

	serverAddr := n.serverNodeAddr + serverInfo.NodeInfo.ID.String()
	maddr, err := warpnet.NewMultiaddr(serverAddr)
	if err != nil {
		return fmt.Errorf("client node: parsing server address: %s", err)
	}

	peerInfo, err := warpnet.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("client node: creating address info: %s", err)
	}
	client, err := libp2p.New(
		libp2p.Identity(n.privKey),
		libp2p.NoListenAddrs,
		libp2p.DisableMetrics(),
		libp2p.DisableRelay(),
		libp2p.Ping(true),
		libp2p.DisableIdentifyAddressDiscovery(),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Transport(tcp.NewTCPTransport),
		// TODO that's initial PSK but it must be updated thru consensus
		libp2p.PrivateNetwork(pnet.PSK(n.psk)),
		libp2p.UserAgent("warpnet-client"),
	)
	if err != nil {
		return fmt.Errorf("client node: init %s", err)
	}

	n.clientNode = client
	client.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, warpnet.PermanentAddrTTL)
	if len(client.Addrs()) != 0 {
		return errors.New("client node must have no addresses")
	}

	n.streamer = stream.NewStreamPool(n.ctx, n.clientNode)

	err = n.pairNodes(peerInfo.ID.String(), serverInfo)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	log.Infoln("client-server nodes paired")
	log.Infoln("client node created:", n.clientNode.ID())
	n.isRunning.Store(true)
	return nil
}

func (n *WarpClientNode) pairNodes(nodeId string, serverInfo domain.AuthNodeInfo) error {
	if n == nil {
		log.Errorln("client node must not be nil")
		return errors.New("client node must not be nil")
	}
	resp, err := n.ClientStream(nodeId, event.PRIVATE_POST_PAIR, serverInfo)
	if err != nil {
		return err
	}

	var errResp event.ErrorResponse
	if _ = json.JSON.Unmarshal(resp, &errResp); errResp.Message != "" {
		return errResp
	}

	return nil
}

func (n *WarpClientNode) IsRunning() bool {
	return n.isRunning.Load()
}

func (n *WarpClientNode) ClientStream(nodeId string, path string, data any) (_ []byte, err error) {
	if n == nil || n.clientNode == nil {
		return nil, errors.New("client node not initialized")
	}
	var bt []byte
	if data != nil {
		var ok bool
		bt, ok = data.([]byte)
		if !ok {
			bt, err = json.JSON.Marshal(data)
			if err != nil {
				return nil, err
			}
		}
	}

	addrInfo, err := peer.AddrInfoFromString(n.serverNodeAddr + nodeId)
	if err != nil {
		return nil, err
	}

	return n.streamer.Send(*addrInfo, stream.WarpRoute(path), bt)
}

func (n *WarpClientNode) Stop() {
	if n == nil || n.clientNode == nil {
		return
	}
	if err := n.clientNode.Close(); err != nil {
		log.Errorf("client node stop fail: %v", err)
	}
	n.clientNode = nil
	n.isRunning.Store(false)
}
