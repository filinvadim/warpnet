package middleware

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/json"
	"io"
	"log"
)

type MiddlewareResolver interface {
	UnwrapStream(s warpnet.WarpStream, fn StreamPoolHandler)
	Authenticate(s warpnet.WarpStream) error
	SetClientID(id warpnet.WarpPeerID)
}

type StreamPoolHandler func([]byte) (any, error)

type WarpMiddleware struct {
	clientPeerID warpnet.WarpPeerID
}

func NewWarpMiddleware() *WarpMiddleware {
	return &WarpMiddleware{""}
}

func (p *WarpMiddleware) SetClientID(id warpnet.WarpPeerID) {
	p.clientPeerID = id
}

func (p *WarpMiddleware) Authenticate(s warpnet.WarpStream) error {
	if p.clientPeerID == "" {
		return errors.New("no client peer ID")
	}
	route := stream.FromPrIDToRoute(s.Protocol())
	if !route.IsPrivate() {
		return nil
	}
	idMatch := p.clientPeerID == s.Conn().RemotePeer()
	if !idMatch {
		log.Printf("middleware: peer ID mismatch for %s %s", s.Protocol(), s.Conn().RemotePeer())
		return errors.New("middleware: peer ID mismatch")
	}

	return nil
}

// TODO refactor
func (p *WarpMiddleware) UnwrapStream(s warpnet.WarpStream, fn StreamPoolHandler) {
	defer func() {
		log.Printf("middleware closed: %s %s", s.Protocol(), s.Conn().RemotePeer())
		_ = s.Close()
	}()

	log.Println("middleware: server stream opened:", s.Protocol(), s.Conn().RemotePeer())

	reader := bufio.NewReader(s)
	data, err := io.ReadAll(reader)
	if err != nil && err != io.EOF {
		log.Printf("middleware: reading from stream: %v", err)
		return
	}

	response, err := fn(data)
	if err != nil {
		msg := fmt.Sprintf("handling %s message: %v\n", s.Protocol(), err)
		log.Println(msg)
		response = []byte(msg)
	}

	encoder := json.JSON.NewEncoder(s)
	if err := encoder.Encode(response); err != nil {
		log.Printf("fail encoding response: %v", err)
	}
}
