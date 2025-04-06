package middleware

import (
	"errors"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"io"
	"time"
)

type middlewareError string

func (e middlewareError) Error() string {
	return string(e)
}
func (e middlewareError) Bytes() []byte {
	return []byte(e)
}

const (
	ErrUnknownClientPeer middlewareError = "auth failed: unknown client peer"
	ErrStreamReadError   middlewareError = "stream reading failed"
	ErrInternalNodeError middlewareError = "internal node error"
)

type WarpHandler func(msg []byte, s warpnet.WarpStream) (any, error)

type WarpMiddleware struct {
	clientNodeID warpnet.WarpPeerID
}

func NewWarpMiddleware() *WarpMiddleware {
	return &WarpMiddleware{""}
}

func (p *WarpMiddleware) LoggingMiddleware(next warpnet.WarpStreamHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		log.Debugf("middleware: server stream opened: %s %s\n", s.Protocol(), s.Conn().RemotePeer())
		before := time.Now()
		next(s)
		after := time.Now()
		log.Debugf(
			"middleware: server stream closed: %s %s, elapsed: %s\n",
			s.Protocol(),
			s.Conn().RemotePeer(),
			after.Sub(before).String(),
		)
	}
}

func (p *WarpMiddleware) AuthMiddleware(next warpnet.WarpStreamHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		if s.Protocol() == event.PRIVATE_POST_PAIR && p.clientNodeID == "" { // first tether client node
			p.clientNodeID = s.Conn().RemotePeer()
			next(s)
			return
		}
		route := stream.FromPrIDToRoute(s.Protocol())
		if route.IsPrivate() && p.clientNodeID == "" {
			log.Errorf("middleware: client peer ID not set, ignoring private route: %s", route)
			_, _ = s.Write(ErrUnknownClientPeer.Bytes())
			return
		}
		if route.IsPrivate() && p.clientNodeID != "" { // not private == no auth
			if !(p.clientNodeID == s.Conn().RemotePeer()) { // only own client node can do private requests
				log.Errorf("middleware: client peer id mismatch: %s", s.Conn().RemotePeer())
				_, _ = s.Write(ErrUnknownClientPeer.Bytes())
				return
			}
		}
		next(s)
	}
}

const (
	KB = 1000
	MB = 1000 * KB
)

func (p *WarpMiddleware) UnwrapStreamMiddleware(fn WarpHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		defer func() { s.Close() }() //#nosec

		var (
			response any
			encoder  = json.JSON.NewEncoder(s)
		)

		reader := io.LimitReader(s, MB) // TODO size limit???
		data, err := io.ReadAll(reader)
		if err != nil && err != io.EOF {
			log.Errorf("middleware: reading from stream: %v", err)
			response = event.ErrorResponse{Message: ErrStreamReadError.Error()}
			return
		}

		if response == nil {
			response, err = fn(data, s)
			if err != nil && !errors.Is(err, warpnet.ErrNodeIsOffline) {
				log.Debugf(">>> STREAM REQUEST %s %s\n", string(s.Protocol()), string(data))
				log.Debugf("<<< STREAM RESPONSE: %s %+v\n", string(s.Protocol()), response)
				log.Errorf("middleware: handling %s message: %s: %v\n", s.Protocol(), string(data), err)
				response = event.ErrorResponse{Code: 500, Message: err.Error()}
			}
		}

		switch response.(type) {
		case []byte:
			if _, err := s.Write(response.([]byte)); err != nil {
				log.Errorf("middleware: writing raw bytes to stream: %v", err)
			}
			return
		case string:
			if _, err := s.Write([]byte(response.(string))); err != nil {
				log.Errorf("middleware: writing string to stream: %v", err)
			}
			return
		default:
			if err := encoder.Encode(response); err != nil {
				log.Errorf("middleware: failed encoding generic response: %v", err)
			}
		}

	}
}
