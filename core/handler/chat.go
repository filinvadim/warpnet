package handler

import (
	"errors"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/gen/event-gen"
	"github.com/filinvadim/warpnet/json"
	"time"
)

type ChatStorer interface {
	CreateChat(chatId *string, ownerId, otherUserId string) (string, error)
	DeleteChat(chatId string) error
	GetUserChats(userID string, limit *uint64, cursor *string) ([]domain.Chat, string, error)
	CreateMessage(msg domain.ChatMessage) (domain.ChatMessage, error)
	ListMessages(chatId string, limit *uint64, cursor *string) ([]domain.ChatMessage, string, error)
	GetMessage(userId, chatId, id string) (domain.ChatMessage, error)
	DeleteMessage(userId, chatId, id string) error
}

// Handler for creating a new chat
func StreamCreateChatHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.NewChatEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.OwnerId == "" || ev.OtherUserId == "" {
			return nil, errors.New("owner ID or other user ID is empty")
		}

		chatId, err := repo.CreateChat(ev.ChatId, ev.OwnerId, ev.OtherUserId)
		if err != nil {
			return nil, err
		}

		return event.ChatCreatedResponse{
			ChatId: chatId,
		}, nil
	}
}

// Handler for deleting a chat
func StreamDeleteChatHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.DeleteChatEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" {
			return nil, errors.New("owner ID, chat ID or other user ID is empty")
		}

		err = repo.DeleteChat(ev.ChatId)
		if err != nil {
			return nil, err
		}
		return event.Accepted, nil
	}
}

// Handler for getting user chats
func StreamGetUserChatsHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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

		return event.ChatsResponse{
			UserId: ev.UserId,
			Chats:  chats,
			Cursor: cursor,
		}, nil
	}
}

// Handler for sending a new message
func StreamSendMessageHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.NewMessageEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" || ev.UserId == "" || ev.Text == "" {
			return nil, errors.New("chat ID, user ID or message text is empty")
		}

		msg := domain.ChatMessage{
			ChatId:    ev.ChatId,
			UserId:    ev.UserId,
			Text:      ev.Text,
			CreatedAt: time.Now(),
		}

		msg, err = repo.CreateMessage(msg)
		if err != nil {
			return nil, err
		}

		return event.Accepted, nil
	}
}

// Handler for getting messages in a chat
func StreamGetMessagesHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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

		return event.MessagesResponse{
			ChatId:   ev.ChatId,
			UserId:   ev.UserId,
			Messages: messages,
			Cursor:   cursor,
		}, nil
	}
}

// Handler for retrieving a specific message
func StreamGetMessageHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
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

		return event.MessageResponse(msg), nil
	}
}

// Handler for deleting a message
func StreamDeleteMessageHandler(repo ChatStorer) middleware.WarpHandler {
	return func(buf []byte) (any, error) {
		var ev event.DeleteMessageEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.ChatId == "" || ev.UserId == "" || ev.Id == "" {
			return nil, errors.New("chat ID, user ID, or message ID cannot be blank")
		}

		err = repo.DeleteMessage(ev.UserId, ev.ChatId, ev.Id)
		if err != nil {
			return nil, err
		}
		return event.Accepted, nil
	}
}
