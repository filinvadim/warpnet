package database

import (
	"testing"

	"github.com/dgraph-io/badger/v3"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock ConsensusStorer (имитация BadgerDB)
type mockConsensusStorer struct {
	data map[string][]byte
}

func newMockConsensusStorer() *mockConsensusStorer {
	return &mockConsensusStorer{data: make(map[string][]byte)}
}

func (m *mockConsensusStorer) Set(key storage.DatabaseKey, value []byte) error {
	m.data[string(key)] = value
	return nil
}

func (m *mockConsensusStorer) Get(key storage.DatabaseKey) ([]byte, error) {
	val, ok := m.data[string(key)]
	if !ok {
		return nil, badger.ErrKeyNotFound
	}
	return val, nil
}

func (m *mockConsensusStorer) Sync() error                                 { return nil }
func (m *mockConsensusStorer) Path() string                                { return "/tmp/mockdb" }
func (m *mockConsensusStorer) InnerDB() *storage.WarpDB                    { return nil }
func (m *mockConsensusStorer) NewWriteTxn() (*storage.WarpWriteTxn, error) { return nil, nil }
func (m *mockConsensusStorer) NewReadTxn() (*storage.WarpReadTxn, error)   { return nil, nil }

func TestConsensusRepo_StoreAndGetLog(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	// Создаем Raft Log
	logEntry := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  []byte("test-data"),
	}

	// Записываем лог
	err := repo.StoreLog(logEntry)
	require.NoError(t, err)

	// Читаем лог
	var retrievedLog raft.Log
	err = repo.GetLog(1, &retrievedLog)
	require.NoError(t, err)

	assert.Equal(t, logEntry.Index, retrievedLog.Index)
	assert.Equal(t, logEntry.Term, retrievedLog.Term)
	assert.Equal(t, logEntry.Type, retrievedLog.Type)
	assert.Equal(t, logEntry.Data, retrievedLog.Data)
}

func TestConsensusRepo_FirstLastIndex(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	// Добавляем несколько логов
	for i := uint64(1); i <= 5; i++ {
		logEntry := &raft.Log{
			Index: i,
			Term:  1,
			Type:  raft.LogCommand,
			Data:  []byte("test"),
		}
		require.NoError(t, repo.StoreLog(logEntry))
	}

	firstIndex, err := repo.FirstIndex()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), firstIndex)

	lastIndex, err := repo.LastIndex()
	require.NoError(t, err)
	assert.Equal(t, uint64(5), lastIndex)
}

func TestConsensusRepo_DeleteRange(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	// Добавляем логи
	for i := uint64(1); i <= 10; i++ {
		logEntry := &raft.Log{Index: i, Term: 1, Data: []byte("data")}
		require.NoError(t, repo.StoreLog(logEntry))
	}

	// Удаляем логи с 3 по 7
	err := repo.DeleteRange(3, 7)
	require.NoError(t, err)

	// Проверяем, что удалены только указанные логи
	for i := uint64(1); i <= 10; i++ {
		var log raft.Log
		err := repo.GetLog(i, &log)
		if i >= 3 && i <= 7 {
			assert.ErrorIs(t, err, ErrConsensusKeyNotFound)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestConsensusRepo_SetGet(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	err := repo.Set([]byte("test-key"), []byte("test-value"))
	require.NoError(t, err)

	val, err := repo.Get([]byte("test-key"))
	require.NoError(t, err)
	assert.Equal(t, []byte("test-value"), val)
}

func TestConsensusRepo_SetGetUint64(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	err := repo.SetUint64([]byte("counter"), 42)
	require.NoError(t, err)

	val, err := repo.GetUint64([]byte("counter"))
	require.NoError(t, err)
	assert.Equal(t, uint64(42), val)
}

func TestConsensusRepo_GetLog_NotFound(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	var log raft.Log
	err := repo.GetLog(99, &log)
	assert.ErrorIs(t, err, ErrConsensusKeyNotFound)
}

func TestConsensusRepo_Get_NotFound(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	_, err := repo.Get([]byte("unknown-key"))
	assert.ErrorIs(t, err, ErrConsensusKeyNotFound)
}

func TestConsensusRepo_DeleteRange_EmptyDB(t *testing.T) {
	mockDB := newMockConsensusStorer()
	repo, _ := NewConsensusRepo(mockDB)

	err := repo.DeleteRange(1, 10)
	assert.NoError(t, err)
}
