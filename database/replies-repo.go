package database

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/filinvadim/warpnet/database/storage"
	domain_gen "github.com/filinvadim/warpnet/domain-gen"
	"github.com/filinvadim/warpnet/json"
	"github.com/google/uuid"
	"sort"
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

func (repo *RepliesRepo) GetReply(rootID, parentID, replyID string) (tweet domain_gen.Tweet, err error) {
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

func (repo *RepliesRepo) GetRepliesTree(rootID, parentID string, limit *uint64, cursor *string) ([]domain_gen.ReplyNode, string, error) {
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

	replies := make([]domain_gen.Tweet, 0, len(items))
	if err = json.JSON.Unmarshal(bytes.Join(items, []byte(",")), &replies); err != nil {
		return nil, "", err
	}
	items = nil

	return buildRepliesTree(replies), cur, nil
}

func buildRepliesTree(replies []domain_gen.Tweet) []domain_gen.ReplyNode {
	nodeMap := make(map[string]domain_gen.ReplyNode, len(replies)) // Карта для хранения всех узлов
	roots := make([]domain_gen.ReplyNode, 0, len(replies))         // Массив для корневых узлов

	for _, reply := range replies { // Создаем узлы для всех твитов
		if reply.Id == "" {
			continue
		}
		nodeMap[reply.Id] = domain_gen.ReplyNode{
			Reply:    reply,
			Children: []domain_gen.ReplyNode{},
		}
	}

	for _, reply := range replies { // Построение дерева
		var (
			node domain_gen.ReplyNode
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

		expandedChildren := make([]domain_gen.ReplyNode, 0, len(parentNode.Children)+1)
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
