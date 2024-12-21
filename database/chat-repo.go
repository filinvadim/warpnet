package database

import (
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/database/storage"
	domainGen "github.com/filinvadim/warpnet/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
	"sort"
	"time"
)

const (
	ChatNamespace       = "CHAT"
	MessageSubNamespace = "MESSAGE"
)

type ChatRepo struct {
	db *storage.DB
}

func NewChatRepo(db *storage.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

func (repo *ChatRepo) CreateChat(userId, otherUserId string) (string, error) {
	chatId := uuid.New().String()

	userKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(userId).
		AddReversedTimestamp(time.Now()).
		AddParentId(chatId).
		Build()
	chatKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(chatId).
		AddReversedTimestamp(time.Now()).
		AddParentId(userId).
		Build()

	chat := domainGen.Chat{
		CreatedAt:  time.Now(),
		Id:         chatId,
		ToUserId:   otherUserId,
		UpdatedAt:  time.Now(),
		FromUserId: userId,
	}

	bt, err := json.JSON.Marshal(chat)
	if err != nil {
		return "", err
	}
	return chatId, repo.db.WriteTxn(func(tx *badger.Txn) error {
		err := repo.db.Set(userKey, bt)
		if err != nil {
			return err
		}
		return repo.db.Set(chatKey, bt)
	})
}

// GetRoom returns the roomIDs for the given userID.
func (repo *ChatRepo) GetChats(userID string) ([]domainGen.Chat, error) {
	if userID == "" {
		return nil, errors.New("ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(userID).
		Build()

	items, _, err := repo.db.List(prefix, nil, nil)
	if err != nil {
		return nil, err
	}

	chats := make([]domainGen.Chat, 0, len(items))
	for _, item := range items {
		var chat domainGen.Chat
		err = json.JSON.Unmarshal(item.Value, &chat)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}

	return chats, nil
}

func (repo *ChatRepo) CreateMessage(msg domainGen.ChatMessage) (domainGen.ChatMessage, error) {
	if msg == (domainGen.ChatMessage{}) {
		return msg, errors.New("nil message")
	}
	if msg.Id == "" {
		msg.Id = uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	fixedKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(msg.ChatId).
		AddRange(storage.FixedRangeKey).
		AddParentId(msg.UserId).
		AddId(msg.Id).
		Build()

	sortableKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(msg.ChatId).
		AddReversedTimestamp(msg.CreatedAt).
		AddParentId(msg.UserId).
		AddId(msg.Id).
		Build()

	data, err := json.JSON.Marshal(msg)
	if err != nil {
		return msg, fmt.Errorf("message marshal: %w", err)
	}

	return msg, repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err = repo.db.Set(fixedKey, data); err != nil {
			return err
		}
		return repo.db.Set(sortableKey, data)
	})
}

func (repo *ChatRepo) ListChats(userId string, limit *uint64, cursor *string) ([]domainGen.ChatMessage, string, error) {
	if userId == "" {
		return nil, "", errors.New("user ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(userId).
		Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	chatsMsgs := make([]domainGen.ChatMessage, 0, len(items))
	for _, item := range items {
		var chatMsg domainGen.ChatMessage
		err = json.JSON.Unmarshal(item.Value, &chatMsg)
		if err != nil {
			return nil, "", err
		}
		chatsMsgs = append(chatsMsgs, chatMsg)
	}
	sort.SliceStable(chatsMsgs, func(i, j int) bool {
		return chatsMsgs[i].CreatedAt.After(chatsMsgs[j].CreatedAt)
	})
	return chatsMsgs, cur, nil
}

func (repo *ChatRepo) ListMessages(chatId string, limit *uint64, cursor *string) ([]domainGen.ChatMessage, string, error) {
	if chatId == "" {
		return nil, "", errors.New("chat ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatId).
		Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	chatsMsgs := make([]domainGen.ChatMessage, 0, len(items))
	for _, item := range items {
		var chatMsg domainGen.ChatMessage
		err = json.JSON.Unmarshal(item.Value, &chatMsg)
		if err != nil {
			return nil, "", err
		}
		chatsMsgs = append(chatsMsgs, chatMsg)
	}
	sort.SliceStable(chatsMsgs, func(i, j int) bool {
		return chatsMsgs[i].CreatedAt.After(chatsMsgs[j].CreatedAt)
	})
	return chatsMsgs, cur, nil
}

func (repo *ChatRepo) GetMessage(userId, chatId, id string) (m domainGen.ChatMessage, err error) {
	fixedKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatId).
		AddRange(storage.FixedRangeKey).
		AddParentId(userId).
		AddId(id).
		Build()
	data, err := repo.db.Get(fixedKey)
	if err != nil {
		return m, err
	}

	err = json.JSON.Unmarshal(data, &m)
	if err != nil {
		return m, err
	}
	data = nil
	return m, nil
}

func (repo *ChatRepo) DeleteMessage(userId, chatId, id string) error {
	m, err := repo.GetMessage(userId, chatId, id)
	if err != nil {
		return err
	}
	fixedKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatId).
		AddRange(storage.FixedRangeKey).
		AddParentId(userId).
		AddId(id).
		Build()
	sortableKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatId).
		AddReversedTimestamp(m.CreatedAt).
		AddParentId(userId).
		AddId(id).
		Build()
	err = repo.db.WriteTxn(func(tx *badger.Txn) error {
		if err = repo.db.Delete(fixedKey); err != nil {
			return err
		}
		return repo.db.Delete(sortableKey)
	})
	return err
}
