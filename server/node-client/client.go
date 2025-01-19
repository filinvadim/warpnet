package node_client

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/filinvadim/warpnet/config"
	"github.com/filinvadim/warpnet/core/encrypting"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
	"log"
)

type ClientNode struct {
	ctx      context.Context
	node     host.Host
	serverId peer.ID
}

func NewNodeClient(ctx context.Context, serverNodeId string, conf config.Config) (*ClientNode, error) {
	libp2p.DefaultMuxers = nil

	log.Println("creating client node")

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
		libp2p.UserAgent(conf.Node.PSK),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client node: %s", err)
	}
	serverAddr := "/ip4/127.0.0.1/tcp/4001/p2p/" + serverNodeId
	maddr, err := ma.NewMultiaddr(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("parsing server address: %s", err)
	}

	serverInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return nil, fmt.Errorf("creating Address info: %s", err)
	}

	log.Println("client node connecting to ", serverInfo.ID)
	if err := client.Connect(context.Background(), *serverInfo); err != nil {
		return nil, fmt.Errorf("connecting to server node: %s", err)
	}
	log.Println("client node connected to server")

	return &ClientNode{
		ctx,
		client,
		serverInfo.ID,
	}, nil
}

func (n *ClientNode) Close() {
	if n == nil {
		return
	}
	if err := n.node.Close(); err != nil {
		log.Printf("closing client node: %s", err)
	}
	return
}

// path = "/example/1.0.0"
func (n *ClientNode) Send(path string, data []byte) ([]byte, error) {
	stream, err := n.node.NewStream(n.ctx, n.serverId, protocol.ID(path))
	if err != nil {
		return nil, fmt.Errorf("opening stream: %s", err)
	}
	defer closeStream(stream)

	var rw = bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	if !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, []byte("\n")...)
	}
	_, err = rw.Write(data)
	if err != nil {
		return nil, fmt.Errorf("writing to stream: %s", err)
	}
	defer flush(rw)

	response, err := rw.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("reading response: %s", err)
	}
	fmt.Printf("Response from server: %s\n", response)
	return response, nil
}

func closeStream(stream network.Stream) {
	if err := stream.Close(); err != nil {
		log.Printf("closing stream: %s", err)
	}
}

func flush(rw *bufio.ReadWriter) {
	if err := rw.Flush(); err != nil {
		log.Printf("flush: %s", err)
	}
}
