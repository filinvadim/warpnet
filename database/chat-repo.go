package database

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
	"strings"
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

type (
	chatID = string
)

func (repo *ChatRepo) CreateChat(chatId *string, ownerId, otherUserId string) (domain.Chat, error) {
	if ownerId == "" || otherUserId == "" {
		return domain.Chat{}, errors.New("user ID or other user ID is empty")
	}
	if err := uuid.Validate(ownerId); err != nil {
		return domain.Chat{}, err
	}

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return domain.Chat{}, err
	}
	defer txn.Rollback()

	chatters := ownerId + ":" + otherUserId
	nonceKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatters).
		Build()

	if chatId == nil || *chatId == "" {
		chatId = new(chatID)
		inc, err := txn.Get(nonceKey)
		if err != nil && !errors.Is(err, storage.ErrKeyNotFound) {
			return domain.Chat{}, err
		}
		*chatId = composeChatId(ownerId, otherUserId, string(inc))
	}

	chatIdKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(*chatId).
		Build()

	fixedUserChatKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(ownerId).
		AddRange(storage.FixedRangeKey).
		AddParentId(*chatId).
		Build()

	sortableUserChatKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(ownerId).
		AddReversedTimestamp(time.Now()).
		AddParentId(*chatId).
		Build()

	chat := domain.Chat{
		CreatedAt:   time.Now(),
		Id:          *chatId,
		OtherUserId: otherUserId,
		UpdatedAt:   time.Now(),
		OwnerId:     ownerId,
	}

	bt, err := json.JSON.Marshal(chat)
	if err != nil {
		return chat, err
	}
	err = txn.Set(fixedUserChatKey, sortableUserChatKey.Bytes())
	if err != nil {
		return chat, err
	}
	err = txn.Set(sortableUserChatKey, bt)
	if err != nil {
		return chat, err
	}
	err = txn.Set(chatIdKey, []byte(chatters))
	if err != nil {
		return chat, err
	}
	return chat, txn.Commit()
}

func (repo *ChatRepo) DeleteChat(chatId string) error {
	if chatId == "" {
		return errors.New("chat ID is empty")
	}

	//ownerId, otherUserId, _ := decomposeChatId(chatId)

	txn, err := repo.db.NewWriteTxn()
	if err != nil {
		return err
	}
	defer txn.Rollback()

	chatIdKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(chatId).
		Build()
	data, err := txn.Get(chatIdKey)
	if err != nil {
		return err
	}

	split := strings.Split(string(data), ":")
	ownerId := split[0]

	fixedUserChatKey := storage.NewPrefixBuilder(ChatNamespace).
		AddRootID(ownerId).
		AddRange(storage.FixedRangeKey).
		AddParentId(chatId).
		Build()

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

	chatters := string(data)
	nonceKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(chatters).
		Build()

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
		AddParentId(msg.OwnerId).
		AddId(msg.Id).
		Build()

	sortableKey := storage.NewPrefixBuilder(ChatNamespace).
		AddSubPrefix(MessageSubNamespace).
		AddRootID(msg.ChatId).
		AddReversedTimestamp(msg.CreatedAt).
		AddParentId(msg.OwnerId).
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

// TODO access this approach
func composeChatId(ownerId, otherUserId, nonce string) string {
	composed := fmt.Sprintf("%s:%s:%s", ownerId, otherUserId, nonce)
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(composed)).String()
}
