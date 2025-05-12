// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

package database

import (
	"go.uber.org/goleak"
	"testing"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/suite"
)

type ChatRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *ChatRepo
}

func (s *ChatRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)
	authRepo := NewAuthRepo(s.db)

	err = authRepo.Authenticate("test", "test")
	s.Require().NoError(err)

	s.repo = NewChatRepo(s.db)
}

func (s *ChatRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *ChatRepoTestSuite) TestCreateAndGetChat() {
	ownerID := ulid.Make().String()
	otherID := ulid.Make().String()

	chat, err := s.repo.CreateChat(nil, ownerID, otherID)
	s.Require().NoError(err)

	fetched, err := s.repo.GetChat(chat.Id)
	s.Require().NoError(err)
	s.Equal(chat.Id, fetched.Id)
	s.Equal(chat.OwnerId, fetched.OwnerId)
	s.Equal(chat.OtherUserId, fetched.OtherUserId)
}

func (s *ChatRepoTestSuite) TestDeleteChat() {
	ownerID := ulid.Make().String()
	otherID := ulid.Make().String()
	chat, err := s.repo.CreateChat(nil, ownerID, otherID)
	s.Require().NoError(err)

	err = s.repo.DeleteChat(chat.Id)
	s.Require().NoError(err)

	deleted, err := s.repo.GetChat(chat.Id)
	s.Require().NoError(err)
	s.Empty(deleted.Id)
}

func (s *ChatRepoTestSuite) TestGetUserChats() {
	userID := ulid.Make().String()
	for i := 0; i < 3; i++ {
		other := ulid.Make().String()
		_, err := s.repo.CreateChat(nil, userID, other)
		s.Require().NoError(err)
	}

	limit := uint64(10)
	chats, cursor, err := s.repo.GetUserChats(userID, &limit, nil)
	s.Require().NoError(err)
	s.Len(chats, 3)
	s.Equal(cursor, "end")
}

func (s *ChatRepoTestSuite) TestCreateAndGetMessage() {
	ownerID := ulid.Make().String()
	otherID := ulid.Make().String()
	chat, _ := s.repo.CreateChat(nil, ownerID, otherID)

	msg := domain.ChatMessage{
		ChatId:  chat.Id,
		OwnerId: ownerID,
		Text:    "hello",
	}

	created, err := s.repo.CreateMessage(msg)
	s.Require().NoError(err)

	got, err := s.repo.GetMessage(ownerID, chat.Id, created.Id)
	s.Require().NoError(err)
	s.Equal(msg.Text, got.Text)
}

func (s *ChatRepoTestSuite) TestListMessages() {
	ownerID := ulid.Make().String()
	otherID := ulid.Make().String()
	chat, _ := s.repo.CreateChat(nil, ownerID, otherID)

	for i := 0; i < 5; i++ {
		msg := domain.ChatMessage{
			ChatId:    chat.Id,
			OwnerId:   ownerID,
			Text:      "msg",
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Second),
		}
		_, err := s.repo.CreateMessage(msg)
		s.Require().NoError(err)
	}

	limit := uint64(10)
	msgs, cursor, err := s.repo.ListMessages(chat.Id, &limit, nil)
	s.Require().NoError(err)
	s.Len(msgs, 5)
	s.Equal(cursor, "end")
}

func (s *ChatRepoTestSuite) TestDeleteMessage() {
	ownerID := ulid.Make().String()
	otherID := ulid.Make().String()
	chat, _ := s.repo.CreateChat(nil, ownerID, otherID)

	msg := domain.ChatMessage{
		ChatId:  chat.Id,
		OwnerId: ownerID,
		Text:    "to delete",
	}
	created, _ := s.repo.CreateMessage(msg)

	err := s.repo.DeleteMessage(ownerID, chat.Id, created.Id)
	s.Require().NoError(err)

	got, err := s.repo.GetMessage(ownerID, chat.Id, created.Id)
	s.Require().NoError(err)
	s.Empty(got.Text)
}

func TestChatRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(ChatRepoTestSuite))
	closeWriter()
}
