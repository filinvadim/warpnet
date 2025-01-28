package database

import (
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database/storage"
	domainGen "github.com/filinvadim/warpnet/gen/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
	"sort"
	"time"
)

const (
	RepliesNamespace = "/REPLY"
)

type ReplyStorer interface {
	WriteTxn(f func(tx *storage.WarpTxn) error) error
	Set(key storage.DatabaseKey, value []byte) error
	List(prefix storage.DatabaseKey, limit *uint64, cursor *string) ([]storage.ListItem, string, error)
	Get(key storage.DatabaseKey) ([]byte, error)
	Delete(key storage.DatabaseKey) error
}

type ReplyRepo struct {
	db ReplyStorer
}

func NewRepliesRepo(db ReplyStorer) *ReplyRepo {
	return &ReplyRepo{db: db}
}

func (repo *ReplyRepo) AddReply(reply domainGen.Tweet) (domainGen.Tweet, error) {
	if reply == (domainGen.Tweet{}) {
		return reply, errors.New("empty reply")
	}
	if reply.RootId == "" {
		return reply, errors.New("empty root")
	}
	if reply.ParentId == "" {
		return reply, errors.New("empty parent")
	}
	if reply.Id == "" {
		reply.Id = uuid.New().String()
	}
	if reply.Id == reply.RootId {
		return reply, errors.New("this is tweet not reply")
	}
	if reply.CreatedAt.IsZero() {
		now := time.Now()
		reply.CreatedAt = now
	}

	metaData, err := json.JSON.Marshal(reply)
	if err != nil {
		return reply, fmt.Errorf("error marshalling reply meta: %w", err)
	}

	treeKey := storage.NewPrefixBuilder(RepliesNamespace).
		AddRootID(reply.RootId).
		AddRange(storage.NoneRangeKey).
		AddParentId(reply.ParentId).
		AddId(reply.Id).
		Build()

	return reply, repo.db.Set(treeKey, metaData)
}

func (repo *ReplyRepo) GetReply(rootID, parentID, replyID string) (tweet domainGen.Tweet, err error) {
	treeKey := storage.NewPrefixBuilder(RepliesNamespace).
		AddRootID(rootID).
		AddRange(storage.NoneRangeKey).
		AddParentId(parentID).
		AddId(replyID).
		Build()
	data, err := repo.db.Get(treeKey)
	if err != nil {
		return tweet, err
	}

	err = json.JSON.Unmarshal(data, &tweet)
	if err != nil {
		return tweet, err
	}
	return tweet, nil
}

func (repo *ReplyRepo) DeleteReply(rootID, parentID, replyID string) error {
	treeKey := storage.NewPrefixBuilder(RepliesNamespace).
		AddRootID(rootID).
		AddRange(storage.NoneRangeKey).
		AddParentId(parentID).
		AddId(replyID).
		Build()
	return repo.db.Delete(treeKey)
}

func (repo *ReplyRepo) GetRepliesTree(rootID, parentID string, limit *uint64, cursor *string) ([]domainGen.ReplyNode, string, error) {
	if rootID == "" {
		return nil, "", errors.New("ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(RepliesNamespace).
		AddRootID(rootID).
		AddRange(storage.NoneRangeKey).
		AddParentId(parentID).
		Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	replies := make([]domainGen.Tweet, 0, len(items))
	for _, item := range items {
		var t domainGen.Tweet
		err = json.JSON.Unmarshal(item.Value, &t)
		if err != nil {
			return nil, "", err
		}
		replies = append(replies, t)
	}
	items = nil

	return buildRepliesTree(replies), cur, nil
}

func buildRepliesTree(replies []domainGen.Tweet) []domainGen.ReplyNode {
	nodeMap := make(map[string]domainGen.ReplyNode, len(replies)) // Карта для хранения всех узлов
	roots := make([]domainGen.ReplyNode, 0, len(replies))         // Массив для корневых узлов

	for _, reply := range replies { // Создаем узлы для всех твитов
		if reply.Id == "" {
			continue
		}
		nodeMap[reply.Id] = domainGen.ReplyNode{
			Reply:    reply,
			Children: []domainGen.ReplyNode{},
		}
	}

	for _, reply := range replies { // Построение дерева
		var (
			node domainGen.ReplyNode
			ok   bool
		)
		if reply.Id != "" {
			node, ok = nodeMap[reply.Id]
		}
		if !ok {
			continue
		}

		if reply.ParentId == "" { // Если ParentId отсутствует, это корневой узел
			roots = append(roots, node)
			continue
		}

		parentNode, ok := nodeMap[reply.ParentId] // Если у твита есть ParentId, проверяем наличие родителя
		if !ok {
			// Если родителя нет, добавляем твит как корневой
			roots = append(roots, node)
			continue
		}

		expandedChildren := make([]domainGen.ReplyNode, 0, len(parentNode.Children)+1)
		copy(expandedChildren, parentNode.Children)
		expandedChildren = append(expandedChildren, node)
		parentNode.Children = expandedChildren // Добавляем в Children родителя

		nodeMap[reply.ParentId] = parentNode
	}

	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].Reply.CreatedAt.After(roots[j].Reply.CreatedAt)
	})

	return roots
}
