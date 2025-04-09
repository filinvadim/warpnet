package database

import (
	log "github.com/ipfs/go-log/writer"
	"go.uber.org/goleak"
	"testing"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type AuthRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *AuthRepo
}

func (s *AuthRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)
	s.repo = NewAuthRepo(s.db)

	err = s.repo.Authenticate("test", "test")
	s.Require().NoError(err)
}

func (s *AuthRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *AuthRepoTestSuite) TestAuthenticate_Success() {
	assert.NotEmpty(s.T(), s.repo.SessionToken())
	assert.NotNil(s.T(), s.repo.PrivateKey())
}

func (s *AuthRepoTestSuite) TestAuthenticate_InvalidCredentials() {
	err := s.repo.Authenticate("", "")
	assert.Error(s.T(), err)
}

func (s *AuthRepoTestSuite) TestAuthenticate_DBNotInitialized() {
	var repo = &AuthRepo{}
	err := repo.Authenticate("test", "test")
	assert.Error(s.T(), err)
}

func (s *AuthRepoTestSuite) TestSessionTokenAndPrivateKey() {
	assert.NotEmpty(s.T(), s.repo.SessionToken())
	assert.NotNil(s.T(), s.repo.PrivateKey())
}

func (s *AuthRepoTestSuite) TestPrivateKey_PanicIfNil() {
	defer func() {
		if r := recover(); r == nil {
			s.T().Errorf("expected panic on nil private key")
		}
	}()
	repo := &AuthRepo{}
	_ = repo.PrivateKey()
}

func (s *AuthRepoTestSuite) TestSetAndGetOwner() {
	owner := domain.Owner{UserId: "owner123"}

	saved, err := s.repo.SetOwner(owner)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "owner123", saved.UserId)

	fetched := s.repo.GetOwner()
	assert.Equal(s.T(), saved.UserId, fetched.UserId)
}

func (s *AuthRepoTestSuite) TestSetOwner_SetsCreatedAt() {
	owner := domain.Owner{UserId: "john"}

	saved, err := s.repo.SetOwner(owner)
	assert.NoError(s.T(), err)
	assert.False(s.T(), saved.CreatedAt.IsZero())
}

func TestAuthRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)
	suite.Run(t, new(AuthRepoTestSuite))
	closeWriter()
}

func closeWriter() {
	defer func() { recover() }()
	_ = log.WriterGroup.Close()
}
