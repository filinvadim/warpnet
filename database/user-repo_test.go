package database_test

import (
	"fmt"
	domain_gen "github.com/filinvadim/warpnet/domain"
	"testing"

	"github.com/filinvadim/warpnet/database"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func setupUserTestDB(t *testing.T) *storage.DB {
	path := "/"
	dir := "tmp"
	// Открываем базу данных в этой директории
	db, err := storage.New(path, true, dir)
	assert.NoError(t, err)
	err = db.Run("test", "test")
	assert.NoError(t, err)
	return db
}

func TestUserRepo_Create(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()
	repo := database.NewUserRepo(db)

	user := domain_gen.User{
		Username: "Test User",
	}
	userID := ulid.Make().String()
	user.Id = userID

	_, err := repo.Create(user)

	assert.NoError(t, err)

	// Проверяем, что пользователь был корректно создан
	retrievedUser, err := repo.Get(userID)
	assert.NoError(t, err)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.Equal(t, user.Id, retrievedUser.Id)
}

func TestUserRepo_Update(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	repo := database.NewUserRepo(db)

	user := domain_gen.User{
		Username: "Test User",
	}
	userID := ulid.Make().String()
	user.Id = userID

	_, err := repo.Create(user)
	assert.NoError(t, err)

	updatedUsername := "Test User Updated"
	_, err = repo.Update(user.Id, domain_gen.User{Username: updatedUsername})
	assert.NoError(t, err)

	// Проверяем, что пользователь был корректно создан
	retrievedUser, err := repo.Get(userID)
	assert.NoError(t, err)
	assert.Equal(t, updatedUsername, retrievedUser.Username)
	assert.Equal(t, user.Id, retrievedUser.Id)
}

func TestUserRepo_Get(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	repo := database.NewUserRepo(db)

	userID := ulid.Make().String()
	user := domain_gen.User{
		Username: "Test User",
		Id:       userID,
	}

	_, err := repo.Create(user)
	assert.NoError(t, err)

	// Проверяем, что пользователь может быть получен
	retrievedUser, err := repo.Get(userID)
	assert.NoError(t, err)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.Equal(t, user.Id, retrievedUser.Id)
}

func TestUserRepo_Delete(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	repo := database.NewUserRepo(db)

	userID := ulid.Make().String()
	user := domain_gen.User{
		Username: "Test User",
		Id:       userID,
	}

	_, err := repo.Create(user)
	assert.NoError(t, err)

	// Удаляем пользователя
	err = repo.Delete(userID)
	assert.NoError(t, err)

	// Проверяем, что пользователь был удален
	_, err = repo.Get(userID)
	assert.Error(t, err)
}

func TestUserRepo_List(t *testing.T) {
	db := setupUserTestDB(t)
	defer db.Close()

	repo := database.NewUserRepo(db)

	userID := ulid.Make().String()
	user1 := domain_gen.User{
		Username: "User1",
		Id:       userID,
	}
	userID2 := ulid.Make().String()
	user2 := domain_gen.User{
		Username: "User2",
		Id:       userID2,
	}

	_, err := repo.Create(user1)
	assert.NoError(t, err)
	_, err = repo.Create(user2)
	assert.NoError(t, err)

	// Получаем список пользователей
	users, _, err := repo.List(nil, nil)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
	fmt.Println(users[0].Username, users[1].Username)

	// Проверяем, что все пользователи корректно получены
	assert.Equal(t, user1.Username, users[0].Username)
	assert.Equal(t, userID, users[0].Id)
	assert.Equal(t, user2.Username, users[1].Username)
	assert.Equal(t, userID2, users[1].Id)
}
