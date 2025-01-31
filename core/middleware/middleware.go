package middleware

import (
	"bufio"
	"github.com/filinvadim/warpnet/core/p2p"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"io"
	"log"
)

type WarpHandler func([]byte) (any, error)

type WarpMiddleware struct {
	clientPeerID warpnet.WarpPeerID
}

func NewWarpMiddleware() *WarpMiddleware {
	return &WarpMiddleware{""}
}

func (p *WarpMiddleware) LoggingMiddleware(next warpnet.WarpStreamHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		log.Printf("middleware: server stream opened: %s %s\n", s.Protocol(), s.Conn().RemotePeer())
		next(s)
		log.Printf("middleware: server stream closed: %s %s\n", s.Protocol(), s.Conn().RemotePeer())
	}
}

func (p *WarpMiddleware) AuthMiddleware(next warpnet.WarpStreamHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		if s.Protocol() == stream.PairPostPrivate.ProtocolID() { // first pair client node
			p.clientPeerID = s.Conn().RemotePeer()
			next(s)
			return
		}
		route := stream.FromPrIDToRoute(s.Protocol())
		if route.IsPrivate() && p.clientPeerID == "" {
			log.Println("middleware: client peer ID not set, ignoring private route:", route)
			s.Write([]byte("auth failed"))
			return
		}
		if route.IsPrivate() && p.clientPeerID != "" { // not private == no auth
			idMatch := p.clientPeerID == s.Conn().RemotePeer() // only own client node can do private requests
			if !idMatch {
				log.Println("middleware: client peer id mismatch:", s.Conn().RemotePeer())
				s.Write([]byte("auth failed"))
				return
			}
		}
		next(s)
	}
}

func (p *WarpMiddleware) UnwrapStreamMiddleware(fn WarpHandler) warpnet.WarpStreamHandler {
	return func(s warpnet.WarpStream) {
		defer func() {
			_ = s.Close()
		}()

		reader := bufio.NewReader(s)
		data, err := io.ReadAll(reader)
		if err != nil && err != io.EOF {
			log.Printf("middleware: reading from stream: %v", err)
			return
		}

		response, err := fn(data)
		if err != nil {
			log.Printf("handling %s message: %v\n", s.Protocol(), err)
			response = domain.Error{Message: err.Error()}
		}

		encoder := json.JSON.NewEncoder(s)

		switch response.(type) {
		case []byte:
			if _, err := s.Write(response.([]byte)); err != nil {
				log.Printf("middleware: writing bytes to stream: %v", err)
			}
			return
		case p2p.NodeInfo:
			info := response.(p2p.NodeInfo)
			info.StreamStats = s.Stat()
			response = info
		default:
		}
		if err := encoder.Encode(response); err != nil {
			log.Printf("fail encoding generic response: %v", err)
		}
	}
}
