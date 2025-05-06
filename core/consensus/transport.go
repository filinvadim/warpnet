package consensus

import (
	"context"
	"errors"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/hashicorp/raft"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/peer"
	"net"
	"time"
)

const raftProtocol warpnet.WarpProtocolID = "/raft/1.0.0/rpc"

type addrProvider struct{}

func (ap *addrProvider) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	return raft.ServerAddress(id), nil

}

type streamLayer struct {
	host NodeServicesProvider
	l    net.Listener
}

func newStreamLayer(h NodeServicesProvider) (*streamLayer, error) {
	listener, err := gostream.Listen(h.Node(), raftProtocol)
	if err != nil {
		return nil, err
	}

	return &streamLayer{
		host: h,
		l:    listener,
	}, nil
}

func (sl *streamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	if sl.host == nil {
		return nil, errors.New("stream layer not initialized")
	}

	pid, err := peer.Decode(string(address))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return gostream.Dial(ctx, sl.host.Node(), pid, raftProtocol)
}

func (sl *streamLayer) Accept() (net.Conn, error) {
	return sl.l.Accept()
}

func (sl *streamLayer) Addr() net.Addr {
	return sl.l.Addr()
}

func (sl *streamLayer) Close() error {
	return sl.l.Close()
}

func NewWarpnetConsensusTransport(node NodeServicesProvider, l *consensusLogger) (*raft.NetworkTransport, error) {
	p2pStream, err := newStreamLayer(node)
	if err != nil {
		return nil, err
	}

	transportConfig := &raft.NetworkTransportConfig{
		ServerAddressProvider: &addrProvider{},
		Logger:                l,
		Stream:                p2pStream,
		MaxPool:               0,
		Timeout:               time.Minute,
	}

	return raft.NewNetworkTransportWithConfig(transportConfig), nil
}
