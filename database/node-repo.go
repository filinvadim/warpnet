package database

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/dWighter/api/server"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
	"github.com/google/uuid"
)

var ErrNodeNotFound = errors.New("node not found")

const NodesRepoName = "NODES"

type NodeRepo struct {
	db *storage.DB
}

func NewNodeRepo(db *storage.DB) *NodeRepo {
	return &NodeRepo{db: db}
}

func (repo *NodeRepo) Create(node server.Node) (uuid.UUID, error) {
	if node.OwnerId == "" {
		return uuid.UUID{}, errors.New("owner id is required")
	}
	if node.Ip == "" {
		return uuid.UUID{}, errors.New("node IP address is missing")
	}
	if node.Id.String() == "" {
		node.Id = uuid.New()
	}

	return node.Id, repo.db.Txn(func(tx *badger.Txn) error {
		ipKey, err := storage.NewPrefixBuilder(NodesRepoName).AddIPAddress(node.Ip).Build()
		if err != nil {
			return err
		}
		userKey, err := storage.NewPrefixBuilder(NodesRepoName).AddUserId(node.OwnerId).Build()
		if err != nil {
			return err
		}
		idKey, err := storage.NewPrefixBuilder(NodesRepoName).AddNodeId(node.Id.String()).Build()
		if err != nil {
			return err
		}

		data, err := json.JSON.Marshal(node)
		if err != nil {
			return err
		}

		err = repo.db.Set(ipKey, data)
		if err != nil {
			return err
		}
		err = repo.db.Set(idKey, data)
		if err != nil {
			return err
		}
		return repo.db.Set(userKey, data)
	})
}

func (repo *NodeRepo) Update(n *server.Node) error {
	if n == nil {
		return errors.New("node is nil")
	}
	if n.Ip == "" {
		return errors.New("node IP address is missing")
	}

	key, err := storage.NewPrefixBuilder(NodesRepoName).AddIPAddress(n.Ip).Build()
	if err != nil {
		return err
	}

	bt, err := json.JSON.Marshal(*n)
	return repo.db.Update(key, bt)
}

func (repo *NodeRepo) GetByIP(ip string) (*server.Node, error) {
	key, err := storage.NewPrefixBuilder(NodesRepoName).AddIPAddress(ip).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	var node server.Node
	err = json.JSON.Unmarshal(data, &node)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (repo *NodeRepo) DeleteByIP(ip string) error {
	node, err := repo.GetByIP(ip)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return ErrNodeNotFound
	}
	if err != nil {
		return err
	}

	return repo.db.Txn(func(tx *badger.Txn) error {
		ipKey, err := storage.NewPrefixBuilder(NodesRepoName).AddIPAddress(node.Ip).Build()
		if err != nil {
			return err
		}
		userKey, err := storage.NewPrefixBuilder(NodesRepoName).AddUserId(node.OwnerId).Build()
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

func (repo *NodeRepo) GetByUserId(userId string) (*server.Node, error) {
	key, err := storage.NewPrefixBuilder(NodesRepoName).AddUserId(userId).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	var node server.Node
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

	return repo.db.Txn(func(tx *badger.Txn) error {
		ipKey, err := storage.NewPrefixBuilder(NodesRepoName).AddIPAddress(node.Ip).Build()
		if err != nil {
			return err
		}
		userKey, err := storage.NewPrefixBuilder(NodesRepoName).AddUserId(node.OwnerId).Build()
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

func (repo *NodeRepo) List() ([]server.Node, error) {
	key, err := storage.NewPrefixBuilder(NodesRepoName).Build()
	if err != nil {
		return nil, err
	}

	nodes := make([]server.Node, 0, 20)
	err = repo.db.IterateKeysValues(key, func(key string, value []byte) error {
		var n server.Node
		err := json.JSON.Unmarshal(value, &n)
		if err != nil {
			return err
		}
		nodes = append(nodes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return nodes, nil
}
