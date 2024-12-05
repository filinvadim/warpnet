package database

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/filinvadim/dWighter/database/storage"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
	"time"
)

const (
	RepliesNamespace = "REPLIES"
)

// TweetRepo handles operations related to tweets
type RepliesRepo struct {
	db *storage.DB
}

func NewRepliesRepo(db *storage.DB) *RepliesRepo {
	return &RepliesRepo{db: db}
}

func (repo *RepliesRepo) AddReply(reply domain_gen.Tweet) (domain_gen.Tweet, error) {
	if reply == (domain_gen.Tweet{}) {
		return reply, errors.New("empty reply")
	}
	if reply.RootId == nil {
		return reply, errors.New("empty root")
	}
	if reply.ParentId == nil {
		return reply, errors.New("empty parent")
	}
	if reply.TweetId == nil {
		id := uuid.New().String()
		reply.TweetId = &id
	}
	if *reply.TweetId == *reply.RootId {
		return reply, errors.New("this is tweet not reply")
	}
	if reply.CreatedAt == nil {
		now := time.Now()
		reply.CreatedAt = &now
	}

	metaData, err := json.JSON.Marshal(reply)
	if err != nil {
		return reply, fmt.Errorf("error marshalling reply meta: %w", err)
	}

	treeKey := storage.NewPrefixBuilder(RepliesNamespace).
		AddRootID(*reply.RootId).
		AddRange(storage.FixedRangeKey).
		AddParentId(*reply.ParentId).
		AddId(*reply.TweetId).
		Build()

	return reply, repo.db.Set(treeKey, metaData)
}

func (repo *RepliesRepo) GetReply(rootID, parentID, tweetID string) (tweet domain_gen.Tweet, err error) {
	treeKey := storage.NewPrefixBuilder(RepliesNamespace).
		AddRootID(rootID).
		AddRange(storage.FixedRangeKey).
		AddParentId(parentID).
		AddId(tweetID).
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

func (repo *RepliesRepo) GetRepliesTree(rootID, parentID string, limit *uint64, cursor *string) ([]ReplyNode, string, error) {
	if rootID == "" {
		return nil, "", errors.New("ID cannot be blank")
	}

	prefix := storage.NewPrefixBuilder(RepliesNamespace).
		AddRootID(rootID).
		AddRange(storage.FixedRangeKey).
		AddParentId(parentID).
		Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	replies := make([]domain_gen.Tweet, 0, len(items))
	if err = json.JSON.Unmarshal(bytes.Join(items, []byte(",")), &replies); err != nil {
		return nil, "", err
	}
	items = nil

	return buildRepliesTree(replies), cur, nil
}

type ReplyNode struct {
	Reply    domain_gen.Tweet `json:"tweet"`
	Children []ReplyNode      `json:"children"`
}

func buildRepliesTree(replies []domain_gen.Tweet) []ReplyNode {
	nodeMap := make(map[string]ReplyNode, len(replies)) // Карта для хранения всех узлов
	roots := make([]ReplyNode, 0, len(replies))         // Массив для корневых узлов

	for _, reply := range replies { // Создаем узлы для всех твитов
		if reply.TweetId == nil {
			continue
		}
		nodeMap[*reply.TweetId] = ReplyNode{
			Reply:    reply,
			Children: []ReplyNode{},
		}
	}

	for _, reply := range replies { // Построение дерева
		var (
			node ReplyNode
			ok   bool
		)
		if reply.TweetId != nil {
			node, ok = nodeMap[*reply.TweetId]
		}
		if !ok {
			continue
		}

		if reply.ParentId == nil { // Если ParentId отсутствует, это корневой узел
			roots = append(roots, node)
			continue
		}

		parentNode, ok := nodeMap[*reply.ParentId] // Если у твита есть ParentId, проверяем наличие родителя
		if !ok {
			// Если родителя нет, добавляем твит как корневой
			roots = append(roots, node)
			continue
		}

		expandedChildren := make([]ReplyNode, 0, len(parentNode.Children)+1)
		copy(expandedChildren, parentNode.Children)
		expandedChildren = append(expandedChildren, node)
		parentNode.Children = expandedChildren // Добавляем в Children родителя

		nodeMap[*reply.ParentId] = parentNode
	}

	return roots
}
