package database

import (
	"errors"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
)

var ErrNodeNotFound = errors.New("node not found")

const (
	NodesRepoName = "NODES"
	KindHost      = "hosts"
)

type NodeRepo struct {
	db      *storage.DB
	ownNode *domain_gen.Node
}

func NewNodeRepo(db *storage.DB) *NodeRepo {
	return &NodeRepo{db: db}
}

func (repo *NodeRepo) Create(node *domain_gen.Node) (*domain_gen.Node, error) {
	if node == nil {
		return nil, errors.New("nil node")
	}
	if node.OwnerId == "" {
		return nil, errors.New("owner id is required")
	}
	if node.Host == "" {
		return nil, errors.New("node host address is missing")
	}
	if node.Id.String() == "" {
		node.Id = uuid.New()
	}
	if node.CreatedAt == nil {
		node.CreatedAt = &time.Time{}
		*node.CreatedAt = time.Now()
	}

	IPKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(KindHost).
		AddReversedTimestamp(*node.CreatedAt).
		AddId(node.Host).
		Build()
	userKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddId(node.OwnerId).
		Build()
	idKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddId(node.Id.String()).
		Build()

	data, err := json.JSON.Marshal(*node)
	if err != nil {
		return nil, err
	}

	err = repo.db.Txn(func(tx *badger.Txn) error {
		err = repo.db.Set(IPKey, data)
		if err != nil {
			return err
		}
		err = repo.db.Set(idKey, data)
		if err != nil {
			return err
		}
		return repo.db.Set(userKey, data)
	})
	if err != nil {
		return nil, err
	}
	if node.IsOwned {
		repo.ownNode = node
	}

	return node, nil
}

func (repo *NodeRepo) OwnNode() *domain_gen.Node {
	return repo.ownNode
}

func (repo *NodeRepo) GetByHost(host string, createdAt time.Time) (*domain_gen.Node, error) {
	IPKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(KindHost).
		AddReversedTimestamp(createdAt).
		AddId(host).
		Build()
	data, err := repo.db.Get(IPKey)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	var node domain_gen.Node
	err = json.JSON.Unmarshal(data, &node)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (repo *NodeRepo) DeleteByHost(host string, createdAt time.Time) error {
	node, err := repo.GetByHost(host, createdAt)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return ErrNodeNotFound
	}
	if err != nil {
		return err
	}

	ipKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(KindHost).
		AddReversedTimestamp(*node.CreatedAt).
		AddId(node.Host).
		Build()
	userKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddId(node.OwnerId).
		Build()
	idKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddId(node.Id.String()).
		Build()

	return repo.db.Txn(func(tx *badger.Txn) error {
		if err != nil {
			return err
		}
		err = repo.db.Delete(ipKey)
		if err != nil {
			return err
		}
		err = repo.db.Delete(idKey)
		if err != nil {
			return err
		}
		return repo.db.Delete(userKey)
	})
}

func (repo *NodeRepo) GetByUserId(userId string) (*domain_gen.Node, error) {
	key := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddId(userId).
		Build()
	data, err := repo.db.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	var node domain_gen.Node
	err = json.JSON.Unmarshal(data, &node)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (repo *NodeRepo) DeleteByUserId(userId string) error {
	node, err := repo.GetByUserId(userId)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return ErrNodeNotFound
	}
	if err != nil {
		return err
	}
	ipKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(KindHost).
		AddReversedTimestamp(*node.CreatedAt).
		AddId(node.Host).
		Build()
	userKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddId(node.OwnerId).
		Build()
	idKey := storage.NewPrefixBuilder(NodesRepoName).
		AddParent(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddId(node.Id.String()).
		Build()
	return repo.db.Txn(func(tx *badger.Txn) error {
		err = repo.db.Delete(idKey)
		if err != nil {
			return err
		}
		err = repo.db.Delete(ipKey)
		if err != nil {
			return err
		}
		return repo.db.Delete(userKey)
	})
}

func (repo *NodeRepo) List(limit *uint64, cursor *string) ([]domain_gen.Node, string, error) {
	if limit == nil {
		limit = new(uint64)
		*limit = 20
	}

	// TODO pagination
	prefix := storage.NewPrefixBuilder(NodesRepoName).AddParent(KindHost).Build()

	if cursor != nil && *cursor != "" {
		prefix = storage.DatabaseKey(*cursor)
	}

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	nodes := make([]domain_gen.Node, 0, *limit)
	err = json.JSON.Unmarshal(items, &nodes)
	if err != nil {
		return nil, "", err
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].CreatedAt.After(*nodes[j].CreatedAt)
	})

	return nodes, cur, nil
}
