package database

import (
	"go.uber.org/goleak"
	"testing"
	"time"

	"github.com/filinvadim/warpnet/database/storage"
	"github.com/filinvadim/warpnet/domain"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/suite"
)

type ReplyRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *ReplyRepo
}

func (s *ReplyRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)

	auth := NewAuthRepo(s.db)
	s.Require().NoError(auth.Authenticate("test", "test"))

	s.repo = NewRepliesRepo(s.db)
}

func (s *ReplyRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *ReplyRepoTestSuite) TestAddAndGetReply() {
	parentID := ulid.Make().String()
	rootID := ulid.Make().String()
	reply := domain.Tweet{
		Id:        ulid.Make().String(),
		RootId:    rootID,
		ParentId:  &parentID,
		UserId:    "user123",
		Text:      "This is a reply",
		CreatedAt: time.Now(),
	}

	saved, err := s.repo.AddReply(reply)
	s.Require().NoError(err)
	s.NotEmpty(saved.Id)

	got, err := s.repo.GetReply(rootID, reply.Id)
	s.Require().NoError(err)
	s.Equal(reply.Text, got.Text)
	s.Equal(reply.UserId, got.UserId)
}

func (s *ReplyRepoTestSuite) TestRepliesCount() {
	parentID := ulid.Make().String()
	rootID := ulid.Make().String()

	for i := 0; i < 3; i++ {
		reply := domain.Tweet{
			Id:        ulid.Make().String(),
			RootId:    rootID,
			ParentId:  &parentID,
			UserId:    "user123",
			Text:      "reply",
			CreatedAt: time.Now(),
		}
		_, err := s.repo.AddReply(reply)
		s.Require().NoError(err)
	}

	count, err := s.repo.RepliesCount(parentID)
	s.Require().NoError(err)
	s.Equal(uint64(3), count)
}

func (s *ReplyRepoTestSuite) TestDeleteReply() {
	parentID := ulid.Make().String()
	rootID := ulid.Make().String()
	replyID := ulid.Make().String()

	reply := domain.Tweet{
		Id:        replyID,
		RootId:    rootID,
		ParentId:  &parentID,
		UserId:    "user123",
		Text:      "to delete",
		CreatedAt: time.Now(),
	}

	_, err := s.repo.AddReply(reply)
	s.Require().NoError(err)

	err = s.repo.DeleteReply(rootID, parentID, replyID)
	s.Require().NoError(err)

	_, err = s.repo.GetReply(rootID, replyID)
	s.Error(err)
}

func (s *ReplyRepoTestSuite) TestGetRepliesTree() {
	rootID := ulid.Make().String()
	parentID := ulid.Make().String()

	// Создаем 3 реплая к parentID
	for i := 0; i < 3; i++ {
		reply := domain.Tweet{
			Id:        ulid.Make().String(),
			RootId:    rootID,
			ParentId:  &parentID,
			UserId:    "user",
			Text:      "child",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		_, err := s.repo.AddReply(reply)
		s.Require().NoError(err)
	}

	limit := uint64(10)
	tree, cursor, err := s.repo.GetRepliesTree(rootID, parentID, &limit, nil)
	s.Require().NoError(err)
	s.Len(tree, 3)
	s.Equal(cursor, "end")

	for _, node := range tree {
		s.Equal("child", node.Reply.Text)
	}
}

func TestReplyRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(ReplyRepoTestSuite))
	closeWriter()
}
