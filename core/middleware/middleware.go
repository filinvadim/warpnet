package middleware

import (
	"fmt"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	log "github.com/sirupsen/logrus"
	"io"
	"strings"
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

type WarpHandler func([]byte) (any, error)

type WarpMiddleware struct {
	clientNodeID warpnet.WarpPeerID
}

func NewWarpMiddleware() *WarpMiddleware {
	return &WarpMiddleware{""}
}

func (p *WarpMiddleware) LoggingMiddleware(next warpnet.WarpStreamHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		log.Infof("middleware: server stream opened: %s %s\n", s.Protocol(), s.Conn().RemotePeer())
		before := time.Now()
		next(s)
		after := time.Now()
		log.Infof(
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
			log.Errorf("middleware: client peer ID not set, ignoring private route:", route)
			s.Write(ErrUnknownClientPeer.Bytes())
			return
		}
		if route.IsPrivate() && p.clientNodeID != "" { // not private == no auth
			if !(p.clientNodeID == s.Conn().RemotePeer()) { // only own client node can do private requests
				log.Errorf("middleware: client peer id mismatch:", s.Conn().RemotePeer())
				s.Write(ErrUnknownClientPeer.Bytes())
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
			response = domain.Error{Message: ErrStreamReadError.Error()}
			return
		}

		if strings.Contains(string(s.Protocol()), "login") ||
			strings.Contains(string(s.Protocol()), "post/user") {
			fmt.Println("OMITTED MESSAGE", string(s.Protocol()))
		} else {
			fmt.Println("MESSAGE", string(s.Protocol()), string(data))
		}

		if response == nil {
			response, err = fn(data)
			if err != nil {
				log.Errorf("handling %s message: %v\n", s.Protocol(), err)
				response = domain.Error{Message: ErrInternalNodeError.Error()}
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
		case p2p.NodeInfo:
			info := response.(p2p.NodeInfo)
			info.StreamStats = s.Stat()
			response = info
		default:
		}
		if err := encoder.Encode(response); err != nil {
			log.Errorf("fail encoding generic response: %v", err)
		}
	}
}
