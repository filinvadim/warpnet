package database

import (
	"testing"
)

func TestChatIDValidity_Success(t *testing.T) {
	ownerId := "bf8dbb8f-916b-4eb6-a6b3-73e5781b37e8"
	otherUserId := "8fb1e522-313b-40c8-82f7-dbfd96cb4853"
	nonce := "1"
	chatId := composeChatId(ownerId, otherUserId, nonce)
	if chatId == "" {
		t.Fatal("ChatId is empty")
	}
}
