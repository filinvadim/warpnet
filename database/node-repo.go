package database

import (
	"errors"
	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/dWighter/api"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/filinvadim/dWighter/json"
)

const NodesRepoName = "NODES"

type NodeRepo struct {
	db *storage.DB
}

func NewNodeRepo(db *storage.DB) *NodeRepo {
	return &NodeRepo{db: db}
}

func (repo *NodeRepo) Create(node api.Node) error {
	if node.OwnerId == "" {
		return errors.New("owner id is required")
	}
	if node.Ip == "" {
		return errors.New("node IP address is missing")
	}
	data, err := json.JSON.Marshal(node)
	if err != nil {
		return err
	}
	_, err = repo.GetByUserId(node.OwnerId)
	if err == nil {
		return errors.New("node already exists")
	}
	if err != badger.ErrKeyNotFound {
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
		err = repo.db.Set(ipKey, data)
		if err != nil {
			return err
		}
		return repo.db.Set(userKey, data)
	})
}

func (repo *NodeRepo) Update(n *api.Node) error {
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

func (repo *NodeRepo) GetByIP(ip string) (*api.Node, error) {
	key, err := storage.NewPrefixBuilder(NodesRepoName).AddIPAddress(ip).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var node api.Node
	err = json.JSON.Unmarshal(data, &node)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (repo *NodeRepo) DeleteByIP(ip string) error {
	node, err := repo.GetByIP(ip)
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

func (repo *NodeRepo) GetByUserId(userId string) (*api.Node, error) {
	key, err := storage.NewPrefixBuilder(NodesRepoName).AddUserId(userId).Build()
	if err != nil {
		return nil, err
	}
	data, err := repo.db.Get(key)
	if err != nil {
		return nil, err
	}

	var node api.Node
	err = json.JSON.Unmarshal(data, &node)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (repo *NodeRepo) DeleteByUserId(userId string) error {
	node, err := repo.GetByUserId(userId)
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

func (repo *NodeRepo) List() ([]api.Node, error) {
	key, err := storage.NewPrefixBuilder(NodesRepoName).Build()
	if err != nil {
		return nil, err
	}

	nodes := make([]api.Node, 0, 20)
	err = repo.db.IterateKeysValues(key, func(key string, value []byte) error {
		var n api.Node
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
