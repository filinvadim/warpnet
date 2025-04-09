package database

import (
	"go.uber.org/goleak"
	"testing"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/stretchr/testify/suite"
)

type UserRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *UserRepo
}

func (s *UserRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)

	authRepo := NewAuthRepo(s.db)
	err = authRepo.Authenticate("test", "test")
	s.Require().NoError(err)

	s.repo = NewUserRepo(s.db)
}

func (s *UserRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *UserRepoTestSuite) TestCreateAndGetUser() {
	user := domain.User{
		Id:        "user1",
		Username:  "testuser",
		CreatedAt: time.Now(),
	}
	created, err := s.repo.Create(user)
	s.Require().NoError(err)
	s.Equal(user.Id, created.Id)

	fetched, err := s.repo.Get(user.Id)
	s.Require().NoError(err)
	s.Equal(user.Id, fetched.Id)
	s.Equal(user.Username, fetched.Username)
}

func (s *UserRepoTestSuite) TestUpdateUser() {
	user := domain.User{
		Id:        "user2",
		Username:  "initial",
		CreatedAt: time.Now(),
	}
	_, err := s.repo.Create(user)
	s.Require().NoError(err)

	updated, err := s.repo.Update(user.Id, domain.User{
		Username: "updated",
	})
	s.Require().NoError(err)
	s.Equal("updated", updated.Username)
}

func (s *UserRepoTestSuite) TestDeleteUser() {
	user := domain.User{
		Id:        "user3",
		Username:  "todelete",
		CreatedAt: time.Now(),
	}
	_, err := s.repo.Create(user)
	s.Require().NoError(err)

	err = s.repo.Delete(user.Id)
	s.Require().NoError(err)

	_, err = s.repo.Get(user.Id)
	s.Equal(ErrUserNotFound, err)
}

func (s *UserRepoTestSuite) TestGetByNodeID() {
	user := domain.User{
		Id:        "user4",
		NodeId:    "node123",
		Username:  "nodeuser",
		CreatedAt: time.Now(),
	}
	_, err := s.repo.Create(user)
	s.Require().NoError(err)

	found, err := s.repo.GetByNodeID("node123")
	s.Require().NoError(err)
	s.Equal("user4", found.Id)
}

func (s *UserRepoTestSuite) TestListAndGetBatch() {
	users := []domain.User{
		{Id: "list1", Username: "a"},
		{Id: "list2", Username: "b"},
		{Id: "list3", Username: "c"},
	}
	for _, u := range users {
		u.CreatedAt = time.Now()
		_, err := s.repo.Create(u)
		s.Require().NoError(err)
	}

	limit := uint64(10)
	all, _, err := s.repo.List(&limit, nil)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(all), 3)

	usersBatch, err := s.repo.GetBatch("list1", "list2")
	s.Require().NoError(err)
	s.Len(usersBatch, 2)
}

func (s *UserRepoTestSuite) TestValidateUserID() {
	user := domain.User{Id: "uniqueUser", CreatedAt: time.Now()}
	_, err := s.repo.Create(user)
	s.Require().NoError(err)

	err = s.repo.ValidateUserID(UserIdConsensusKey, "uniqueUser")
	s.Equal(ErrUserAlreadyExists, err)

	err = s.repo.ValidateUserID(UserIdConsensusKey, "nonexistent")
	s.NoError(err)
}

func TestUserRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(UserRepoTestSuite))
	closeWriter()
}
