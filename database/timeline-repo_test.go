/*

 Warpnet - Decentralized Social Network
 Copyright (C) 2025 Vadim Filin, https://github.com/filinvadim,
 <github.com.mecdy@passmail.net>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU General Public License for more details.

 You should have received a copy of the GNU General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

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

type TimelineRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *TimelineRepo
}

func (s *TimelineRepoTestSuite) SetupSuite() {
	var err error
	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)

	auth := NewAuthRepo(s.db)
	s.Require().NoError(auth.Authenticate("test", "test"))

	s.repo = NewTimelineRepo(s.db)
}

func (s *TimelineRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *TimelineRepoTestSuite) TestAddAndGetTimeline() {
	userID := ulid.Make().String()

	tweet := domain.Tweet{
		Id:        ulid.Make().String(),
		UserId:    userID,
		Text:      "Hello timeline",
		CreatedAt: time.Now(),
	}

	err := s.repo.AddTweetToTimeline(userID, tweet)
	s.Require().NoError(err)

	limit := uint64(10)
	timeline, cursor, err := s.repo.GetTimeline(userID, &limit, nil)
	s.Require().NoError(err)
	s.Len(timeline, 1)
	s.Equal(cursor, "end")
	s.Equal(tweet.Text, timeline[0].Text)
	s.Equal(tweet.Id, timeline[0].Id)
}

func (s *TimelineRepoTestSuite) TestDeleteTweetFromTimeline() {
	userID := ulid.Make().String()

	tweet := domain.Tweet{
		Id:        ulid.Make().String(),
		UserId:    userID,
		Text:      "To be deleted",
		CreatedAt: time.Now(),
	}

	err := s.repo.AddTweetToTimeline(userID, tweet)
	s.Require().NoError(err)

	err = s.repo.DeleteTweetFromTimeline(userID, tweet.Id, tweet.CreatedAt)
	s.Require().NoError(err)

	limit := uint64(10)
	timeline, _, err := s.repo.GetTimeline(userID, &limit, nil)
	s.Require().NoError(err)
	s.Len(timeline, 0)
}

func (s *TimelineRepoTestSuite) TestMultipleTweetsOrder() {
	userID := ulid.Make().String()
	var tweets []domain.Tweet

	for i := 0; i < 3; i++ {
		t := domain.Tweet{
			Id:        ulid.Make().String(),
			UserId:    userID,
			Text:      "tweet",
			CreatedAt: time.Now().Add(time.Duration(-i) * time.Second),
		}
		tweets = append(tweets, t)
		s.Require().NoError(s.repo.AddTweetToTimeline(userID, t))
	}

	limit := uint64(10)
	timeline, _, err := s.repo.GetTimeline(userID, &limit, nil)
	s.Require().NoError(err)
	s.Len(timeline, 3)

	// ensure order: newest to oldest
	s.True(timeline[0].CreatedAt.After(timeline[1].CreatedAt) || timeline[0].CreatedAt.Equal(timeline[1].CreatedAt))
	s.True(timeline[1].CreatedAt.After(timeline[2].CreatedAt) || timeline[1].CreatedAt.Equal(timeline[2].CreatedAt))
}

func TestTimelineRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(TimelineRepoTestSuite))
	closeWriter()
}
