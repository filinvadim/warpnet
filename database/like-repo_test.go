package database

import (
	"go.uber.org/goleak"
	"testing"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/suite"
)

type LikeRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *LikeRepo
}

func (s *LikeRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)

	authRepo := NewAuthRepo(s.db)
	err = authRepo.Authenticate("test", "test")
	s.Require().NoError(err)

	s.repo = NewLikeRepo(s.db)
}

func (s *LikeRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *LikeRepoTestSuite) TestLikeAndUnlike() {
	userId := ulid.Make().String()
	tweetId := ulid.Make().String()

	// Like
	likes, err := s.repo.Like(tweetId, userId)
	s.Require().NoError(err)
	s.Equal(uint64(1), likes)

	// Like again (should not increment)
	likes, err = s.repo.Like(tweetId, userId)
	s.Require().NoError(err)
	s.Equal(uint64(1), likes)

	// Check count directly
	count, err := s.repo.LikesCount(tweetId)
	s.Require().NoError(err)
	s.Equal(uint64(1), count)

	// Check likers
	limit := uint64(10)
	likers, cur, err := s.repo.Likers(tweetId, &limit, nil)
	s.Require().NoError(err)
	s.Len(likers, 1)
	s.Equal(cur, "end")
	s.Equal(userId, likers[0])

	// Unlike
	likes, err = s.repo.Unlike(userId, tweetId)
	s.Require().NoError(err)
	s.Equal(uint64(0), likes)

	// Unlike again (should not fail)
	likes, err = s.repo.Unlike(userId, tweetId)
	s.Require().NoError(err)
	s.Equal(uint64(0), likes)

	// Check likers now
	likers, _, err = s.repo.Likers(tweetId, &limit, nil)
	s.Require().NoError(err)
	s.Len(likers, 0)
}

func (s *LikeRepoTestSuite) TestLike_InvalidParams() {
	tweetId := ulid.Make().String()
	userId := ulid.Make().String()

	_, err := s.repo.Like("", userId)
	s.Error(err)

	_, err = s.repo.Like(tweetId, "")
	s.Error(err)

	_, err = s.repo.Unlike("", userId)
	s.Error(err)

	_, err = s.repo.Unlike(tweetId, "")
	s.Error(err)

	_, err = s.repo.LikesCount("")
	s.Error(err)

	_, _, err = s.repo.Likers("", nil, nil)
	s.Error(err)
}

func (s *LikeRepoTestSuite) TestLikesCount_NotFound() {
	id := ulid.Make().String()
	_, err := s.repo.LikesCount(id)
	s.EqualError(err, ErrLikesNotFound.Error())
}

func (s *LikeRepoTestSuite) TestLikers_Empty() {
	tweetId := ulid.Make().String()
	limit := uint64(10)
	likers, cur, err := s.repo.Likers(tweetId, &limit, nil)
	s.Require().NoError(err)
	s.Empty(likers)
	s.Equal(cur, "end")
}

func TestLikeRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(LikeRepoTestSuite))
	closeWriter()
}
