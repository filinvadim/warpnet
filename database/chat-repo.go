package database

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
	"sort"
	"time"
)

const (
	ChatNamespace       = "/CHATS"
	MessageSubNamespace = "MESSAGES"
)

type ChatStorer interface {
	NewWriteTxn() (*storage.WarpWriteTxn, error)
	NewReadTxn() (*storage.WarpReadTxn, error)
}

type ChatRepo struct {
	db ChatStorer
}

func NewChatRepo(db ChatStorer) *ChatRepo {
	return &ChatRepo{db: db}
}

type chatID = string

func (repo *ChatRepo) CreateChat(chatId, ownerId, otherUserId string) (chatID, error) {
	if ownerId == "" || otherUserId == "" {
		return "", errors.New("user ID or other user ID is empty")
	}

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return "", err
	}
	defer txn.Rollback()

	nonceRootId := ownerId + otherUserId
	nonceKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(nonceRootId).
		Build()

	if chatId == "" {
		inc, err := txn.Get(nonceKey)
		if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
			return chatId, err
		}
		chatId = generateUUIDv5ChatId(ownerId, otherUserId, string(inc))
	}

	fixedUserChatKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(ownerId).
		AddRange(storage.FixedRangeKey).
		AddParentId(chatId).
		Build()

	sortableUserChatKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(ownerId).
		AddReversedTimestamp(time.Now()).
		AddParentId(chatId).
		Build()

	chat := domain.Chat{
		CreatedAt: time.Now(),
		Id:        chatId,
		ToUserId:  otherUserId,
		UpdatedAt: time.Now(),
		OwnerId:   ownerId,
	}

	bt, err := json.JSON.Marshal(chat)
	if err != nil {
		return "", err
	}
	err = txn.Set(fixedUserChatKey, sortableUserChatKey.Bytes())
	if err != nil {
		return "", err
	}
	err = txn.Set(sortableUserChatKey, bt)
	if err != nil {
		return "", err
	}
	return chatId, txn.Commit()
}

func (repo *ChatRepo) DeleteChat(chatId, ownerId, otherUserId string) error {
	if ownerId == "" || chatId == "" || otherUserId == "" {
		return errors.New("user ID or other chat ID is empty")
	}
	fixedUserChatKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(ownerId).
		AddRange(storage.FixedRangeKey).
		AddParentId(chatId).
		Build()

	nonceRootId := ownerId + otherUserId
	nonceKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(nonceRootId).
		Build()

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	sortableKey, err := txn.Get(fixedUserChatKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := txn.Delete(fixedUserChatKey); err != nil {
		return err
	}
	if err := txn.Delete(storage.DatabaseKey(sortableKey)); err != nil {
		return err
	}

	if _, err = txn.Increment(nonceKey); err != nil {
		return err
	}
	return txn.Commit()
}

func (repo *ChatRepo) GetUserChats(userID string, limit *uint64, cursor *string) ([]domain.Chat, string, error) {
	if userID == "" {
		return nil, "", errors.New("ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(userID).
		Build()

	txn, err := repo.db.NewReadTxn()
	if err != nil {
		return nil, "", err
	}
	defer txn.Rollback()

	items, cur, err := txn.List(prefix, limit, cursor)
	if err != nil {
		return nil, cur, err
	}

	if err := txn.Commit(); err != nil {
		return nil, cur, err
	}

	chats := make([]domain.Chat, 0, len(items))
	for _, item := range items {
		var chat domain.Chat
		err = json.JSON.Unmarshal(item.Value, &chat)
		if err != nil {
			return nil, cur, err
		}
		chats = append(chats, chat)
	}

	return chats, cur, nil
}

func (repo *ChatRepo) CreateMessage(msg domain.ChatMessage) (domain.ChatMessage, error) {
	if msg == (domain.ChatMessage{}) {
		return msg, errors.New("empty message")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.Id == "" {
		msg.Id = uuid.New().String()
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
		return msg, fmt.Errorf("message: marshal: %w", err)
	}

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return msg, err
	}
	defer txn.Rollback()

	if err = txn.Set(fixedKey, sortableKey.Bytes()); err != nil {
		return msg, err
	}
	err = txn.Set(sortableKey, data)
	if err != nil {
		return msg, err
	}
	return msg, txn.Commit()
}

func (repo *ChatRepo) ListMessages(chatId string, limit *uint64, cursor *string) ([]domain.ChatMessage, string, error) {
	if chatId == "" {
		return nil, "", errors.New("chat ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatId).
		Build()

	txn, err := repo.db.NewReadTxn()
	if err != nil {
		return nil, "", err
	}
	defer txn.Rollback()

	items, cur, err := txn.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	if err := txn.Commit(); err != nil {
		return nil, cur, err
	}

	chatsMsgs := make([]domain.ChatMessage, 0, len(items))
	for _, item := range items {
		var chatMsg domain.ChatMessage
		err = json.JSON.Unmarshal(item.Value, &chatMsg)
		if err != nil {
			return nil, "", err
		}
		chatsMsgs = append(chatsMsgs, chatMsg)
	}

	return chatsMsgs, cur, nil
}

func (repo *ChatRepo) GetMessage(userId, chatId, id string) (m domain.ChatMessage, err error) {
	fixedKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatId).
		AddRange(storage.FixedRangeKey).
		AddParentId(userId).
		AddId(id).
		Build()

	txn, err := repo.db.NewReadTxn()
	if err != nil {
		return m, err
	}
	defer txn.Rollback()

	sortableKey, err := txn.Get(fixedKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return m, nil
	}
	if err != nil {
		return m, err
	}

	data, err := txn.Get(storage.DatabaseKey(sortableKey))
	if err != nil {
		return m, err
	}

	if err := txn.Commit(); err != nil {
		return m, err
	}

	err = json.JSON.Unmarshal(data, &m)
	if err != nil {
		return m, err
	}
	return m, nil
}

func (repo *ChatRepo) DeleteMessage(userId, chatId, id string) error {
	fixedKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatId).
		AddRange(storage.FixedRangeKey).
		AddParentId(userId).
		AddId(id).
		Build()

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	sortableKey, err := txn.Get(fixedKey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	if err = txn.Delete(fixedKey); err != nil {
		return err
	}
	if err = txn.Delete(storage.DatabaseKey(sortableKey)); err != nil {
		return err
	}

	return txn.Commit()
}

func generateUUIDv5ChatId(userId1, userId2, nonce string) string {
	items := []string{userId1, userId2}
	sort.Strings(items) // Упорядочиваем, чтобы избежать зависимости от порядка

	namespace := uuid.NameSpaceDNS
	return uuid.NewSHA1(namespace, []byte(items[0]+items[1]+nonce)).String()
}
