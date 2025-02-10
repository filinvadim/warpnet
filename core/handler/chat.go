package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"time"
)

type ChatStreamer interface {
	GenericStream(nodeId warpnet.WarpPeerID, path stream.WarpRoute, data any) (_ []byte, err error)
}

type ChatUserFetcher interface {
	GetByNodeID(nodeID string) (user domain.User, err error)
	Get(userId string) (user domain.User, err error)
}

type ChatStorer interface {
	CreateChat(chatId *string, ownerId, otherUserId string) (domain.Chat, error)
	DeleteChat(chatId string) error
	GetUserChats(userId string, limit *uint64, cursor *string) ([]domain.Chat, string, error)
	CreateMessage(msg domain.ChatMessage) (domain.ChatMessage, error)
	ListMessages(chatId string, limit *uint64, cursor *string) ([]domain.ChatMessage, string, error)
	GetMessage(userId, chatId, id string) (domain.ChatMessage, error)
	DeleteMessage(userId, chatId, id string) error
}

// Handler for creating a new chat
func StreamCreateChatHandler(repo ChatStorer, userRepo ChatUserFetcher, streamer ChatStreamer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewChatEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.OwnerId == "" || ev.OtherUserId == "" {
			return nil, errors.New("owner ID or other user ID is empty")
		}

		otherUser, err := userRepo.Get(ev.OtherUserId)
		if err != nil {
			return nil, err
		}

		ownerChat, err := repo.CreateChat(ev.ChatId, ev.OwnerId, ev.OtherUserId)
		if err != nil {
			return nil, err
		}

		otherChatData, err := streamer.GenericStream(
			warpnet.WarpPeerID(otherUser.NodeId),
			event.PUBLIC_POST_CHAT,
			domain.Chat{
				CreatedAt:   time.Now(),
				Id:          ownerChat.Id,
				OtherUserId: ownerChat.OwnerId, // switch users
				OwnerId:     ownerChat.OtherUserId,
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if err := json.JSON.Unmarshal(otherChatData, &possibleError); err == nil {
			return nil, fmt.Errorf("create other chat stream: %s", possibleError.Message)
		}

		return event.ChatCreatedResponse(ownerChat), err
	}
}

// Handler for deleting a chat
func StreamDeleteChatHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.DeleteChatEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" {
			return nil, errors.New("owner ID, chat ID or other user ID is empty")
		}

		return event.Accepted, repo.DeleteChat(ev.ChatId)
	}
}

// Handler for getting user chats
func StreamGetUserChatsHandler(repo ChatStorer, userRepo ChatUserFetcher) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetAllChatsEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user ID")
		}

		chats, cursor, err := repo.GetUserChats(ev.UserId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}

		if len(chats) == 0 {
			return event.ChatsResponse{
				Chats:  []domain.Chat{},
				Cursor: "",
				UserId: ev.UserId,
			}, nil
		}

		var (
			ownerId      = chats[0].OwnerId
			userId       = chats[0].OtherUserId
			remotePeerId = s.Conn().RemotePeer()
		)

		user, err := userRepo.GetByNodeID(remotePeerId.String())
		if err != nil {
			return nil, err
		}

		if user.Id != userId || userId != ownerId {
			return nil, errors.New("unauthorized user")
		}

		return event.ChatsResponse{
			UserId: ev.UserId,
			Chats:  chats,
			Cursor: cursor,
		}, nil
	}
}

// Handler for sending a new message
func StreamSendMessageHandler(repo ChatStorer, userRepo ChatUserFetcher, streamer ChatStreamer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewMessageEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" || ev.OwnerId == "" || ev.Text == "" || ev.OtherUserId == "" {
			return nil, errors.New("message parameters are is empty")
		}

		otherUser, err := userRepo.Get(ev.OtherUserId)
		if err != nil {
			return nil, err
		}

		msg := domain.ChatMessage{
			ChatId:      ev.ChatId,
			OwnerId:     ev.OwnerId,
			OtherUserId: ev.OtherUserId,
			Text:        ev.Text,
			CreatedAt:   time.Now(),
		}

		msg, err = repo.CreateMessage(msg)

		otherMsgData, err := streamer.GenericStream(
			warpnet.WarpPeerID(otherUser.NodeId),
			event.PUBLIC_POST_MESSAGE,
			event.NewMessageEvent{
				CreatedAt:   time.Now(),
				Id:          msg.Id,
				OtherUserId: msg.OwnerId, // switch users
				OwnerId:     msg.OtherUserId,
				Text:        msg.Text,
				ChatId:      msg.ChatId,
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if err := json.JSON.Unmarshal(otherMsgData, &possibleError); err == nil {
			return nil, fmt.Errorf("create other message stream: %s", possibleError.Message)
		}

		return event.NewMessageResponse(msg), err
	}
}

// Handler for deleting a message
func StreamDeleteMessageHandler(repo ChatStorer, userRepo ChatUserFetcher, streamer ChatStreamer) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.DeleteMessageEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" || ev.UserId == "" || ev.Id == "" {
			return nil, errors.New("chat ID, user ID, or message ID cannot be blank")
		}

		return event.Accepted, repo.DeleteMessage(ev.UserId, ev.ChatId, ev.Id)
	}
}

// Handler for getting messages in a chat
func StreamGetMessagesHandler(repo ChatStorer, userRepo ChatUserFetcher) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetAllMessagesEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" {
			return nil, errors.New("chat ID cannot be blank")
		}

		messages, cursor, err := repo.ListMessages(ev.ChatId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, err
		}

		if len(messages) == 0 {
			return event.ChatMessagesResponse{
				ChatId:   ev.ChatId,
				Cursor:   "",
				Messages: []domain.ChatMessage{},
				OwnerId:  ev.OwnerId,
			}, nil
		}
		var (
			ownerId      = messages[0].OwnerId
			userId       = messages[0].OtherUserId
			remotePeerId = s.Conn().RemotePeer()
		)

		user, err := userRepo.GetByNodeID(remotePeerId.String())
		if err != nil {
			return nil, err
		}

		if user.Id != userId || userId != ownerId {
			return nil, errors.New("unauthorized user")
		}

		return event.ChatMessagesResponse{
			ChatId:   ev.ChatId,
			OwnerId:  ev.OwnerId,
			Messages: messages,
			Cursor:   cursor,
		}, nil
	}
}

// Handler for retrieving a specific message
func StreamGetMessageHandler(repo ChatStorer, userRepo ChatUserFetcher) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetMessageEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" || ev.UserId == "" || ev.Id == "" {
			return nil, errors.New("chat ID, user ID, or message ID cannot be blank")
		}

		msg, err := repo.GetMessage(ev.UserId, ev.ChatId, ev.Id)
		if err != nil {
			return nil, err
		}

		var (
			ownerId      = msg.OwnerId
			userId       = msg.OtherUserId
			remotePeerId = s.Conn().RemotePeer()
		)

		user, err := userRepo.GetByNodeID(remotePeerId.String())
		if err != nil {
			return nil, err
		}

		if user.Id != userId || userId != ownerId {
			return nil, errors.New("unauthorized user")
		}

		return event.ChatMessageResponse(msg), nil
	}
}
