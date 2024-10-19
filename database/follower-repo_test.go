package database_test

import (
	"os"
	"testing"

	"github.com/filinvadim/dWighter/database"
	"github.com/filinvadim/dWighter/database/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupFollowTestDB(t *testing.T) *storage.DB {
	path := "../var/dbtestfollow"
	// Открываем базу данных в этой директории
	db := storage.New(path, true)
	db.Run("", "")

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(path)
	})

	return db
}

func TestFollowRepo_Follow(t *testing.T) {
	db := setupFollowTestDB(t)
	repo := database.NewFollowRepo(db)

	readerID := uuid.New().String()
	writerID := uuid.New().String()

	// Создаем связь reader -> writer
	err := repo.Follow(readerID, writerID)
	assert.NoError(t, err)

	// Проверяем, что reader подписан на writer
	writers, err := repo.GetWriters(readerID)
	assert.NoError(t, err)
	assert.Contains(t, writers, writerID)

	// Проверяем, что writer имеет reader
	readers, err := repo.GetReaders(writerID)
	assert.NoError(t, err)
	assert.Contains(t, readers, readerID)
}

func TestFollowRepo_Unfollow(t *testing.T) {
	db := setupFollowTestDB(t)
	repo := database.NewFollowRepo(db)

	readerID := uuid.New().String()
	writerID := uuid.New().String()

	// Создаем связь reader -> writer
	err := repo.Follow(readerID, writerID)
	assert.NoError(t, err)

	// Удаляем связь reader -> writer
	err = repo.Unfollow(readerID, writerID)
	assert.NoError(t, err)

	// Проверяем, что связь удалена
	writers, err := repo.GetWriters(readerID)
	assert.NoError(t, err)
	assert.NotContains(t, writers, writerID)

	readers, err := repo.GetReaders(writerID)
	assert.NoError(t, err)
	assert.NotContains(t, readers, readerID)
}

func TestFollowRepo_GetWriters(t *testing.T) {
	db := setupFollowTestDB(t)
	repo := database.NewFollowRepo(db)

	readerID := "readerid"
	writerID1 := "writerid1"
	writerID2 := "writerid2"

	// Создаем связи reader -> writer1 и reader -> writer2
	err := repo.Follow(readerID, writerID1)
	assert.NoError(t, err)
	err = repo.Follow(readerID, writerID2)
	assert.NoError(t, err)

	// Проверяем, что оба writer'а получены
	writers, err := repo.GetWriters(readerID)
	assert.NoError(t, err)
	assert.Len(t, writers, 2)
	assert.Contains(t, writers, writerID1)
	assert.Contains(t, writers, writerID2)
}

func TestFollowRepo_GetReaders(t *testing.T) {
	db := setupFollowTestDB(t)
	repo := database.NewFollowRepo(db)

	readerID1 := uuid.New().String()
	readerID2 := uuid.New().String()
	writerID := uuid.New().String()

	// Создаем связи reader1 -> writer и reader2 -> writer
	err := repo.Follow(readerID1, writerID)
	assert.NoError(t, err)
	err = repo.Follow(readerID2, writerID)
	assert.NoError(t, err)

	// Проверяем, что оба reader'а получены
	readers, err := repo.GetReaders(writerID)
	assert.NoError(t, err)
	assert.Len(t, readers, 2)
	assert.Contains(t, readers, readerID1)
	assert.Contains(t, readers, readerID2)
}
