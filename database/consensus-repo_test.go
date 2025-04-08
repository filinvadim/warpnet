package database

import (
	"go.uber.org/goleak"
	"testing"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/stretchr/testify/suite"
)

type ConsensusRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *ConsensusRepo
}

func (s *ConsensusRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)
	authRepo := NewAuthRepo(s.db)

	err = authRepo.Authenticate("test", "test")
	s.Require().NoError(err)
	s.repo = NewConsensusRepo(s.db)
}

func (s *ConsensusRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *ConsensusRepoTestSuite) TestSetAndGet() {
	key := []byte("config-key")
	value := []byte("config-value")

	err := s.repo.Set(key, value)
	s.Require().NoError(err)

	got, err := s.repo.Get(key)
	s.Require().NoError(err)
	s.Equal(value, got)
}

func (s *ConsensusRepoTestSuite) TestGet_NotFound() {
	_, err := s.repo.Get([]byte("nonexistent"))
	s.EqualError(err, ErrConsensusKeyNotFound.Error())
}

func (s *ConsensusRepoTestSuite) TestSetAndGetUint64() {
	key := []byte("counter")
	var expected uint64 = 42

	err := s.repo.SetUint64(key, expected)
	s.Require().NoError(err)

	actual, err := s.repo.GetUint64(key)
	s.Require().NoError(err)
	s.Equal(expected, actual)
}

func (s *ConsensusRepoTestSuite) TestGetUint64_DefaultZero() {
	val, err := s.repo.GetUint64([]byte("missing-counter"))
	s.Require().NoError(err)
	s.Equal(uint64(0), val)
}

func (s *ConsensusRepoTestSuite) TestPath() {
	path := s.repo.Path()
	s.Contains(path, "/snapshots")
}

func (s *ConsensusRepoTestSuite) TestSync() {
	err := s.repo.Sync()
	s.NoError(err)
}

func TestConsensusRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(ConsensusRepoTestSuite))
}
