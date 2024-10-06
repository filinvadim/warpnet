package database_test

import (
	domain_gen "github.com/filinvadim/dWighter/domain-gen"
	"os"
	"testing"

	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupNodeTestDB(t *testing.T) *storage.DB {
	path := "../var/dbtestnodes"
	db := storage.New(path, false, "error")

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(path)
	})

	return db
}

func TestNodeRepo_Create(t *testing.T) {
	db := setupNodeTestDB(t)
	repo := database.NewNodeRepo(db)

	node := &domain_gen.Node{
		Host:    "192.168.1.1:16969",
		OwnerId: uuid.New().String(),
	}

	// Create new node
	_, err := repo.Create(node)
	assert.NoError(t, err)

	// Attempt to create the same node again (should fail)
	_, err = repo.Create(node)
	assert.Error(t, err)
	assert.Equal(t, "node already exists", err.Error())

	// Verify node was created correctly by IP
	retrievedNode, err := repo.GetByHost(node.Host)
	assert.NoError(t, err)
	assert.Equal(t, node.Host, retrievedNode.Host)
	assert.Equal(t, node.OwnerId, retrievedNode.OwnerId)

	// Verify node was created correctly by UserId
	retrievedNode, err = repo.GetByUserId(node.OwnerId)
	assert.NoError(t, err)
	assert.Equal(t, node.Host, retrievedNode.Host)
	assert.Equal(t, node.OwnerId, retrievedNode.OwnerId)
}

func TestNodeRepo_GetByIP(t *testing.T) {
	db := setupNodeTestDB(t)
	repo := database.NewNodeRepo(db)

	node := &domain_gen.Node{
		Host:    "10.0.0.1:16969",
		OwnerId: uuid.New().String(),
	}

	// Create a new node
	_, err := repo.Create(node)
	assert.NoError(t, err)

	// Retrieve the node by its IP address
	retrievedNode, err := repo.GetByHost(node.Host)
	assert.NoError(t, err)
	assert.Equal(t, node.Host, retrievedNode.Host)
	assert.Equal(t, node.OwnerId, retrievedNode.OwnerId)

	// Attempt to retrieve a non-existent IP
	_, err = repo.GetByHost("192.168.1.100")
	assert.Error(t, err)
}

func TestNodeRepo_DeleteByIP(t *testing.T) {
	db := setupNodeTestDB(t)
	repo := database.NewNodeRepo(db)

	node := &domain_gen.Node{
		Host:    "10.0.0.2:16969",
		OwnerId: uuid.New().String(),
	}

	// Create a new node
	_, err := repo.Create(node)
	assert.NoError(t, err)

	// Delete the node by its IP address
	err = repo.DeleteByHost(node.Host)
	assert.NoError(t, err)

	// Verify the node no longer exists
	_, err = repo.GetByHost(node.Host)
	assert.Error(t, err)

	// Verify the node is also removed by UserId
	_, err = repo.GetByUserId(node.OwnerId)
	assert.Error(t, err)
}

func TestNodeRepo_DeleteByUserId(t *testing.T) {
	db := setupNodeTestDB(t)
	repo := database.NewNodeRepo(db)

	node := &domain_gen.Node{
		Host:    "10.0.0.3:16969",
		OwnerId: uuid.New().String(),
	}

	// Create a new node
	_, err := repo.Create(node)
	assert.NoError(t, err)

	// Delete the node by its UserId
	err = repo.DeleteByUserId(node.OwnerId)
	assert.NoError(t, err)

	// Verify the node no longer exists by IP
	_, err = repo.GetByHost(node.Host)
	assert.Error(t, err)

	// Verify the node no longer exists by UserId
	_, err = repo.GetByUserId(node.OwnerId)
	assert.Error(t, err)
}
