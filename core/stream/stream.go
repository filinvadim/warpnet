package stream

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/warpnet"
	log "github.com/sirupsen/logrus"
)

type NodeStreamer interface {
	NewStream(ctx context.Context, p warpnet.WarpPeerID, pids ...warpnet.WarpProtocolID) (warpnet.WarpStream, error)
}

type streamPool struct {
	ctx          context.Context
	n            NodeStreamer
	clientPeerID warpnet.WarpPeerID
}

func NewStreamPool(
	ctx context.Context,
	n NodeStreamer,
) *streamPool {
	pool := &streamPool{ctx: ctx, n: n}

	return pool
}

func (p *streamPool) Send(peerAddr warpnet.PeerAddrInfo, r WarpRoute, data []byte) ([]byte, error) {
	if p == nil {
		return nil, nil
	}

	return send(p.ctx, p.n, peerAddr, r, data)
}

func send(
	ctx context.Context, n NodeStreamer,
	serverInfo warpnet.PeerAddrInfo, r WarpRoute, data []byte,
) ([]byte, error) {
	if n == nil || serverInfo.String() == "" || r == "" {
		return nil, errors.New("stream: parameters improperly configured")
	}

	if err := serverInfo.ID.Validate(); err != nil {
		return nil, err
	}

	log.Infof("stream: opening %s to %s...", r.ProtocolID(), serverInfo.ID)
	stream, err := n.NewStream(ctx, serverInfo.ID, r.ProtocolID())
	if err != nil {
		return nil, fmt.Errorf("stream: new: %s", err)
	}
	defer closeStream(stream)

	var rw = bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	if data != nil {
		log.Infof("stream: sent to %s data with size %d\n", r, len(data))
		_, err = rw.Write(data)
	}
	flush(rw)
	closeWrite(stream)
	if err != nil {
		log.Errorf("stream: writing: %v", err)
		return nil, fmt.Errorf("stream: writing: %s", err)
	}

	log.Infoln("stream: waiting for response...", r.ProtocolID(), serverInfo.ID)
	buf := bytes.NewBuffer(nil)
	_, err = buf.ReadFrom(rw)
	if err != nil {
		log.Errorf("stream: reading response: %v", err)
		return nil, fmt.Errorf("reading response: %s", err)
	}
	log.Infof("stream: received response from %s, size %d\n", r, buf.Len())

	return buf.Bytes(), nil
}

func closeStream(stream warpnet.WarpStream) {
	if err := stream.Close(); err != nil {
		log.Infof("closing stream: %s", err)
	}
}

func flush(rw *bufio.ReadWriter) {
	if err := rw.Flush(); err != nil {
		log.Infof("flush: %s", err)
	}
}

func closeWrite(s warpnet.WarpStream) {
	if err := s.CloseWrite(); err != nil {
		log.Infof("close write: %s", err)
	}
}
