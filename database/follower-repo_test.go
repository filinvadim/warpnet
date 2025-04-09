package database

import (
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"testing"
)

type FollowRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *FollowRepo
}

func (s *FollowRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)

	authRepo := NewAuthRepo(s.db)
	err = authRepo.Authenticate("test", "test")
	s.Require().NoError(err)

	s.repo = NewFollowRepo(s.db)
}

func (s *FollowRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *FollowRepoTestSuite) TestFollowAndUnfollow() {
	userA := ulid.Make().String()
	userB := ulid.Make().String()

	following := domain.Following{
		Follower: userA,
		Followee: userB,
	}

	// Follow
	err := s.repo.Follow(userA, userB, following)
	s.Require().NoError(err)

	// Check counts
	countFollowers, err := s.repo.GetFollowersCount(userA)
	s.Require().NoError(err)
	s.Equal(uint64(1), countFollowers)

	countFollowees, err := s.repo.GetFolloweesCount(userB)
	s.Require().NoError(err)
	s.Equal(uint64(1), countFollowees)

	// Check followers
	limit := uint64(10)
	followers, _, err := s.repo.GetFollowers(userB, &limit, nil)
	s.Require().NoError(err)
	s.Len(followers, 1)
	s.Equal(userA, followers[0].Follower)

	// Check followees
	followees, _, err := s.repo.GetFollowees(userA, &limit, nil)
	s.Require().NoError(err)
	s.Len(followees, 1)
	s.Equal(userB, followees[0].Followee)

	// Unfollow
	err = s.repo.Unfollow(userA, userB)
	s.Require().NoError(err)

	// Check counts after unfollowing
	countFollowers, err = s.repo.GetFollowersCount(userA)
	s.Require().NoError(err)
	s.Equal(uint64(0), countFollowers)

	countFollowees, err = s.repo.GetFolloweesCount(userB)
	s.Require().NoError(err)
	s.Equal(uint64(0), countFollowees)

	// Check empty lists
	followers, _, err = s.repo.GetFollowers(userB, &limit, nil)
	s.Require().NoError(err)
	s.Len(followers, 0)

	followees, _, err = s.repo.GetFollowees(userA, &limit, nil)
	s.Require().NoError(err)
	s.Len(followees, 0)
}

func (s *FollowRepoTestSuite) TestFollow_InvalidInput() {
	err := s.repo.Follow("", "userB", domain.Following{})
	s.Error(err)

	err = s.repo.Follow("userA", "", domain.Following{})
	s.Error(err)
}

func (s *FollowRepoTestSuite) TestGetFollowersCount_Empty() {
	count, err := s.repo.GetFollowersCount("")
	s.Error(err)
	s.Equal(uint64(0), count)
}

func (s *FollowRepoTestSuite) TestGetFolloweesCount_Empty() {
	count, err := s.repo.GetFolloweesCount("")
	s.Error(err)
	s.Equal(uint64(0), count)
}

func TestFollowRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(FollowRepoTestSuite))
	closeWriter()
}
