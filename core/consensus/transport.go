package consensus

import (
	"context"
	"errors"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/hashicorp/raft"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"net"
	"time"
)

const raftProtocol warpnet.WarpProtocolID = "/raft/1.0.0/rpc"

type addrProvider struct {
	node NodeServicesProvider
}

func (ap *addrProvider) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	if result := warpnet.FromStringToPeerID(string(id)); result.String() == "" {
		return "", errors.New("raft-transport: invalid server id")
	}

	return raft.ServerAddress(id), nil

}

type streamLayer struct {
	host    NodeServicesProvider
	l       net.Listener
	lg      *consensusLogger
	retrier retrier.Retrier
}

func newStreamLayer(h NodeServicesProvider, lg *consensusLogger) (*streamLayer, error) {
	listener, err := gostream.Listen(h.Node(), raftProtocol)
	if err != nil {
		return nil, err
	}

	return &streamLayer{
		host:    h,
		l:       listener,
		lg:      lg,
		retrier: retrier.New(time.Second*1, 5, retrier.ArithmeticalBackoff),
	}, nil
}

func (sl *streamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	if sl.host == nil {
		return nil, errors.New("stream layer not initialized")
	}

	pid := warpnet.FromStringToPeerID(string(address))
	if pid.String() == "" {
		return nil, errors.New("raft-transport: invalid server id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := gostream.Dial(ctx, sl.host.Node(), pid, raftProtocol)
	if err != nil {
		sl.lg.Debug("raft-transport: dial failed to " + string(address) + ": " + err.Error())
	}
	return conn, err
}

func (sl *streamLayer) Accept() (conn net.Conn, err error) {
	err = sl.retrier.Try(context.Background(), func() error {
		conn, err = sl.l.Accept()
		if err != nil {
			sl.lg.Debug("raft-transport: accept: " + err.Error())
		}
		return err
	})

	return conn, err
}

func (sl *streamLayer) Addr() net.Addr {
	return sl.l.Addr()
}

func (sl *streamLayer) Close() error {
	sl.lg.Debug("raft-trasport: stream closed")
	return sl.l.Close()
}

// Libp2p multiplexes streams over persistent TCP connections,
// so we disable connection pooling (MaxPool=0) to avoid unnecessary complexity.
// Check https://github.com/hashicorp/raft/net_transport.go#L606
const maxPoolConnections int = 0 //

func NewWarpnetConsensusTransport(node NodeServicesProvider, l *consensusLogger) (*raft.NetworkTransport, error) {
	p2pStream, err := newStreamLayer(node, l)
	if err != nil {
		return nil, err
	}

	transportConfig := &raft.NetworkTransportConfig{
		ServerAddressProvider: &addrProvider{node},
		Logger:                l,
		Stream:                p2pStream,
		MaxPool:               maxPoolConnections,
		Timeout:               time.Minute,
	}

	return raft.NewNetworkTransportWithConfig(transportConfig), nil
}
