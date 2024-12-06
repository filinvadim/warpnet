package database

import (
	"bytes"
	"errors"
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
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

func (repo *NodeRepo) Create(node domain_gen.Node) (domain_gen.Node, error) {
	if node == (domain_gen.Node{}) {
		return node, errors.New("nil node")
	}
	if node.OwnerId == "" {
		return node, errors.New("owner id is required")
	}
	if node.Host == "" {
		return node, errors.New("node host address is missing")
	}
	if node.Id.String() == "" {
		node.Id = uuid.New()
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}

	IPKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(KindHost).
		AddReversedTimestamp(node.CreatedAt).
		AddParentId(node.Host).
		Build()
	userKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddParentId(node.OwnerId).
		Build()
	idKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddParentId(node.Id.String()).
		Build()

	data, err := json.JSON.Marshal(node)
	if err != nil {
		return node, err
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
		return node, err
	}
	if node.IsOwned {
		repo.ownNode = &node
	}
	data = nil
	return node, nil
}

func (repo *NodeRepo) OwnNode() *domain_gen.Node {
	return repo.ownNode
}

func (repo *NodeRepo) GetByHost(host string, createdAt time.Time) (node domain_gen.Node, err error) {
	IPKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(KindHost).
		AddReversedTimestamp(createdAt).
		AddParentId(host).
		Build()
	data, err := repo.db.Get(IPKey)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return node, ErrNodeNotFound
	}
	if err != nil {
		return node, err
	}

	err = json.JSON.Unmarshal(data, &node)
	if err != nil {
		return node, err
	}
	data = nil

	return node, nil
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
		AddRootID(KindHost).
		AddReversedTimestamp(node.CreatedAt).
		AddParentId(node.Host).
		Build()
	userKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddParentId(node.OwnerId).
		Build()
	idKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddParentId(node.Id.String()).
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

func (repo *NodeRepo) GetByUserId(userId string) (node domain_gen.Node, err error) {
	key := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddParentId(userId).
		Build()
	data, err := repo.db.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return node, ErrNodeNotFound
	}
	if err != nil {
		return node, err
	}

	err = json.JSON.Unmarshal(data, &node)
	if err != nil {
		return node, err
	}
	return node, nil
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
		AddRootID(KindHost).
		AddReversedTimestamp(node.CreatedAt).
		AddParentId(node.Host).
		Build()
	userKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddParentId(node.OwnerId).
		Build()
	idKey := storage.NewPrefixBuilder(NodesRepoName).
		AddRootID(storage.FixedKey).
		AddRange(storage.FixedRangeKey).
		AddParentId(node.Id.String()).
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
	// TODO pagination
	prefix := storage.NewPrefixBuilder(NodesRepoName).AddRootID(KindHost).Build()

	items, cur, err := repo.db.List(prefix, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	nodes := make([]domain_gen.Node, 0, len(items))
	err = json.JSON.Unmarshal(bytes.Join(items, []byte(",")), &nodes)
	if err != nil {
		return nil, "", err
	}

	return nodes, cur, nil
}
