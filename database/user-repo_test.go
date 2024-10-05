package database_test

import (
	"os"
	"testing"

	"github.com/filinvadim/dWighter/api/components"
	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupUserTestDB(t *testing.T) *storage.DB {
	path := "../var/dbtestuser"
	// Открываем базу данных в этой директории
	db := storage.New(path, true, "error")

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(path)
	})

	return db
}

func TestUserRepo_Create(t *testing.T) {
	db := setupUserTestDB(t)
	repo := database.NewUserRepo(db)

	user := &components.User{
		Username: "Test User",
	}
	userID := uuid.New().String()
	user.UserId = &userID

	_, err := repo.Create(user)

	assert.NoError(t, err)

	// Проверяем, что пользователь был корректно создан
	retrievedUser, err := repo.Get(userID)
	assert.NoError(t, err)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.Equal(t, user.UserId, retrievedUser.UserId)
}

func TestUserRepo_Get(t *testing.T) {
	db := setupUserTestDB(t)
	repo := database.NewUserRepo(db)

	userID := uuid.New().String()
	user := &components.User{
		Username: "Test User",
		UserId:   &userID,
	}

	_, err := repo.Create(user)
	assert.NoError(t, err)

	// Проверяем, что пользователь может быть получен
	retrievedUser, err := repo.Get(userID)
	assert.NoError(t, err)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.Equal(t, user.UserId, retrievedUser.UserId)
}

func TestUserRepo_Delete(t *testing.T) {
	db := setupUserTestDB(t)
	repo := database.NewUserRepo(db)

	userID := uuid.New().String()
	user := &components.User{
		Username: "Test User",
		UserId:   &userID,
	}

	_, err := repo.Create(user)
	assert.NoError(t, err)

	// Удаляем пользователя
	err = repo.Delete(userID)
	assert.NoError(t, err)

	// Проверяем, что пользователь был удален
	retrievedUser, err := repo.Get(userID)
	assert.Error(t, err)
	assert.Nil(t, retrievedUser)
}

func TestUserRepo_List(t *testing.T) {
	db := setupUserTestDB(t)
	repo := database.NewUserRepo(db)

	userID := uuid.New().String()
	user1 := &components.User{
		Username: "User1",
		UserId:   &userID,
	}
	userID = uuid.New().String()
	user2 := &components.User{
		Username: "User2",
		UserId:   &userID,
	}

	_, err := repo.Create(user1)
	assert.NoError(t, err)
	_, err = repo.Create(user2)
	assert.NoError(t, err)

	// Получаем список пользователей
	users, err := repo.List()
	assert.NoError(t, err)
	assert.Len(t, users, 2)

	// Проверяем, что все пользователи корректно получены
	assert.Contains(t, users, user1)
	assert.Contains(t, users, user2)
}
