package handler

import (
	"fmt"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"log"
	"net/http"
)

func handleError(err error) []byte {
	if err == nil {
		return nil
	}

	var errEvent event.ErrorEvent
	errMsg := fmt.Sprintf("fail unmarshaling timeline event: %v", err)
	log.Println(errMsg)

	errEvent = event.ErrorEvent{
		Code:    http.StatusInternalServerError,
		Message: errMsg,
	}

	bt, err := json.JSON.Marshal(errEvent)
	if err != nil {
		log.Printf("fail marshaling response: %v", err)
		return []byte(err.Error())
	}

	return bt
}
