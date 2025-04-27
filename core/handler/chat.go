package handler

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/core/middleware"
	"github.com/filinvadim/warpnet/core/stream"
	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/domain"
	"github.com/filinvadim/warpnet/event"
	"github.com/filinvadim/warpnet/json"
	"strings"
	"time"
)

type ChatAuthStorer interface {
	GetOwner() domain.Owner
}

type ChatStreamer interface {
	GenericStream(nodeId string, path stream.WarpRoute, data any) (_ []byte, err error)
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
	GetChat(chatId string) (chat domain.Chat, err error)
	ComposeChatId(ownerId, otherUserId string) string
}

// Handler for creating a new chat
func StreamCreateChatHandler(
	repo ChatStorer,
	userRepo ChatUserFetcher,
	authRepo ChatAuthStorer,
	streamer ChatStreamer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.NewChatEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}
		if ev.OwnerId == "" || ev.OtherUserId == "" {
			return nil, errors.New("owner ID or other user ID is empty")
		}

		owner := authRepo.GetOwner()
		if ev.ChatId != nil && ev.OtherUserId == owner.UserId {
			*ev.ChatId = repo.ComposeChatId(ev.OtherUserId, ev.OwnerId)
		}
		otherUser, err := userRepo.Get(ev.OtherUserId)
		if err != nil {
			return nil, err
		}

		if otherUser.IsOffline {
			return nil, warpnet.ErrUserIsOffline // TODO
		}

		ownerChat, err := repo.CreateChat(ev.ChatId, ev.OwnerId, ev.OtherUserId)
		if err != nil {
			return nil, err
		}

		otherChatData, err := streamer.GenericStream(
			otherUser.NodeId,
			event.PUBLIC_POST_CHAT,
			domain.Chat{
				CreatedAt: ownerChat.CreatedAt,
				// switch users
				Id:          repo.ComposeChatId(ownerChat.OtherUserId, ownerChat.OwnerId),
				OtherUserId: ownerChat.OwnerId,
				OwnerId:     ownerChat.OtherUserId,
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(otherChatData, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other chat error response: %s", possibleError.Message)
		}

		return event.ChatCreatedResponse(ownerChat), err
	}
}

func StreamGetUserChatHandler(
	repo ChatStorer,
	authRepo ChatAuthStorer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetChatEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, err
		}

		if ev.ChatId == "" {
			return nil, errors.New("empty other chat ID")
		}

		if !strings.Contains(ev.ChatId, authRepo.GetOwner().UserId) {
			return nil, errors.New("not owner's chats")
		}

		chat, err := repo.GetChat(ev.ChatId)
		if err != nil {
			return nil, err
		}

		return event.GetChatResponse(chat), nil
	}
}

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

type OwnerChatsStorer interface {
	GetOwner() domain.Owner
}

func StreamGetUserChatsHandler(
	repo ChatStorer, authRepo OwnerChatsStorer,
) middleware.WarpHandler {
	return func(buf []byte, s warpnet.WarpStream) (any, error) {
		var ev event.GetAllChatsEvent
		err := json.JSON.Unmarshal(buf, &ev)
		if err != nil {
			return nil, fmt.Errorf("get chats: unmarshal: %w", err)
		}
		if ev.UserId == "" {
			return nil, errors.New("empty user ID")
		}

		owner := authRepo.GetOwner()
		if owner.UserId != ev.UserId {
			return nil, errors.New("not owner's chats")
		}

		chats, cursor, err := repo.GetUserChats(ev.UserId, ev.Limit, ev.Cursor)
		if err != nil {
			return nil, fmt.Errorf("get chats: fetch from db: %w", err)
		}

		for i, chat := range chats {
			if !strings.Contains(chat.Id, owner.UserId) {
				chats[i] = domain.Chat{}
			}
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
		if ev.ChatId == "" || !strings.Contains(ev.ChatId, ":") || ev.Text == "" {
			return nil, errors.New("message parameters are invalid")
		}

		split := strings.Split(ev.ChatId, ":")
		ownerId, otherUserId := split[0], split[1]

		msg := domain.ChatMessage{
			ChatId:      ev.ChatId,
			OwnerId:     ownerId,
			OtherUserId: otherUserId,
			Text:        ev.Text,
			CreatedAt:   time.Now(),
		}

		msg, err = repo.CreateMessage(msg)
		if err != nil {
			return nil, err
		}

		otherUser, err := userRepo.Get(otherUserId)
		if err != nil {
			return nil, err
		}

		otherMsgData, err := streamer.GenericStream(
			otherUser.NodeId,
			event.PUBLIC_POST_MESSAGE,
			event.NewMessageEvent{
				CreatedAt: time.Now(),
				Id:        msg.Id,
				Text:      msg.Text,
				ChatId:    repo.ComposeChatId(otherUserId, ownerId), // switch users
			},
		)
		if err != nil {
			return nil, err
		}

		var possibleError event.ErrorResponse
		if _ = json.JSON.Unmarshal(otherMsgData, &possibleError); possibleError.Message != "" {
			return nil, fmt.Errorf("unmarshal other message error response: %s", possibleError.Message)
		}

		return event.NewMessageResponse(msg), err
	}
}

// Handler for deleting a message
func StreamDeleteMessageHandler(repo ChatStorer) middleware.WarpHandler {
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
				Cursor:   cursor,
				Messages: []domain.ChatMessage{},
				OwnerId:  ev.OwnerId,
			}, nil
		}
		var (
			ownerId      = messages[0].OwnerId
			userId       = messages[0].OtherUserId
			remotePeerId = s.Conn().RemotePeer()
		)

		storedUser, err := userRepo.GetByNodeID(remotePeerId.String())
		if err != nil && !errors.Is(err, database.ErrUserNotFound) {
			return nil, fmt.Errorf("remote peer %s: %w", remotePeerId.String(), err)
		}

		// just double check
		if storedUser.Id != "" && (storedUser.Id != userId || storedUser.Id != ownerId) {
			return nil, errors.New("user doesn't own these messages")
		}

		return event.ChatMessagesResponse{
			ChatId:   ev.ChatId,
			OwnerId:  ev.OwnerId,
			Messages: messages,
			Cursor:   cursor,
		}, nil
	}
}

// StreamGetMessageHandler for retrieving a specific message
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
