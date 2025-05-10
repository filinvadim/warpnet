package consensus

import (
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/retrier"
	"github.com/hashicorp/raft"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"net"
	"sync"
	"time"
)

const raftProtocol warpnet.WarpProtocolID = "/raft/1.0.0/rpc"

type addrProvider struct{}

func (ap *addrProvider) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	return raft.ServerAddress(id), nil
}

type streamLayer struct {
	host    NodeTransporter
	l       net.Listener
	lg      *consensusLogger
	retrier retrier.Retrier
	cache   *sync.Map
}

func newStreamLayer(h NodeTransporter, lg *consensusLogger) (*streamLayer, error) {
	peerIdCache := new(sync.Map)

	listener, err := gostream.Listen(h.Node(), raftProtocol)
	if err != nil {
		return nil, err
	}

	return &streamLayer{
		host:    h,
		l:       listener,
		lg:      lg,
		retrier: retrier.New(time.Second*1, 5, retrier.ArithmeticalBackoff),
		cache:   peerIdCache,
	}, nil
}

func (sl *streamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	if sl.host == nil {
		return nil, errors.New("stream layer not initialized")
	}
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	var pid warpnet.WarpPeerID
	value, ok := sl.cache.Load(string(address))
	if ok {
		pid = value.(warpnet.WarpPeerID)
	} else {
		pid = warpnet.FromStringToPeerID(string(address))
		sl.cache.Store(string(address), pid)
	}
	if pid.String() == "" {
		return nil, errors.New("raft-transport: invalid server id")
	}

	fmt.Println(pid.String(), "???????????????????????????????? DIAL")

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
	sl.lg.Debug("raft-transport: stream closed")
	return sl.l.Close()
}

// Libp2p multiplexes streams over persistent TCP connections,
// so we disable connection pooling (MaxPool=0) to avoid unnecessary complexity.
// Check https://github.com/hashicorp/raft/net_transport.go#L606
const maxPoolConnections int = 0 //

func NewWarpnetConsensusTransport(node NodeTransporter, l *consensusLogger) (*raft.NetworkTransport, error) {
	p2pStream, err := newStreamLayer(node, l)
	if err != nil {
		return nil, err
	}

	transportConfig := &raft.NetworkTransportConfig{
		ServerAddressProvider: &addrProvider{},
		Logger:                l,
		Stream:                p2pStream,
		MaxPool:               maxPoolConnections,
		Timeout:               time.Minute,
	}

	return raft.NewNetworkTransportWithConfig(transportConfig), nil
}
