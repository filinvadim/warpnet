package handler

import (
	"bufio"
	"fmt"
	"github.com/filinvadim/warpnet/json"
	"github.com/libp2p/go-libp2p/core/network"
	"io"
	"log"
)

type streamHandler func([]byte) (any, error)

func ReadStream(s network.Stream, fn streamHandler) {
	defer func() {
		log.Printf("stream closed for peer: %s", s.Conn().RemotePeer())
		s.Close()
	}()

	log.Println("server stream opened", s.Protocol(), s.Conn().RemotePeer())

	reader := bufio.NewReader(s)
	data, err := io.ReadAll(reader)
	if err != nil && err != io.EOF {
		log.Printf("error reading from stream: %v", err)
		return
	}
	
	response, err := fn(data)
	if err != nil {
		msg := fmt.Sprintf("error handling %s message: %v\n", s.Protocol(), err)
		log.Println(msg)
		response = []byte(msg)
	}

	encoder := json.JSON.NewEncoder(s)
	if err := encoder.Encode(response); err != nil {
		log.Printf("fail encoding response: %v", err)
	}
}
