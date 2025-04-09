package database

import (
	"context"
	"github.com/filinvadim/warpnet/security"
	"go.uber.org/goleak"
	"testing"
	"time"

	"github.com/filinvadim/warpnet/core/warpnet"
	"github.com/filinvadim/warpnet/database/storage"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/stretchr/testify/suite"
)

type NodeRepoTestSuite struct {
	suite.Suite
	db   *storage.DB
	repo *NodeRepo
	ctx  context.Context
}

func (s *NodeRepoTestSuite) SetupSuite() {
	var err error
	s.ctx = context.Background()

	s.db, err = storage.New(".", true, "")
	s.Require().NoError(err)

	auth := NewAuthRepo(s.db)
	s.Require().NoError(auth.Authenticate("test", "test"))

	s.repo = NewNodeRepo(s.db)
}

func (s *NodeRepoTestSuite) TearDownSuite() {
	s.db.Close()
}

func (s *NodeRepoTestSuite) TestPutGetHasDelete() {
	key := datastore.NewKey("test/key")
	value := []byte("hello")

	err := s.repo.Put(s.ctx, key, value)
	s.Require().NoError(err)

	got, err := s.repo.Get(s.ctx, key)
	s.Require().NoError(err)
	s.Equal(value, got)

	has, err := s.repo.Has(s.ctx, key)
	s.Require().NoError(err)
	s.True(has)

	err = s.repo.Delete(s.ctx, key)
	s.Require().NoError(err)

	_, err = s.repo.Get(s.ctx, key)
	s.ErrorIs(err, datastore.ErrNotFound)
}

func (s *NodeRepoTestSuite) TestPutWithTTLAndSetTTL() {
	key := datastore.NewKey("ttl/key")
	value := []byte("expiring")

	err := s.repo.PutWithTTL(s.ctx, key, value, time.Second*2)
	s.Require().NoError(err)

	// overwrite ttl
	time.Sleep(time.Second)
	err = s.repo.SetTTL(s.ctx, key, time.Second*5)
	s.Require().NoError(err)

	got, err := s.repo.Get(s.ctx, key)
	s.Require().NoError(err)
	s.Equal(value, got)
}

func (s *NodeRepoTestSuite) TestDiskUsage() {
	_, err := s.repo.DiskUsage(s.ctx)
	s.Require().NoError(err)
}

func (s *NodeRepoTestSuite) TestBlocklist() {
	pk, err := security.GenerateKeyFromSeed([]byte("peer123"))
	s.Require().NoError(err)

	warpPrivKey := pk.(warpnet.WarpPrivateKey)
	id, err := warpnet.IDFromPrivateKey(warpPrivKey)
	s.Require().NoError(err)

	err = s.repo.Blocklist(s.ctx, id)
	s.Require().NoError(err)

	isBlocked, err := s.repo.IsBlocklisted(s.ctx, id)
	s.Require().NoError(err)
	s.True(isBlocked)

	err = s.repo.BlocklistRemove(s.ctx, id)
	s.Require().NoError(err)

	isBlocked, err = s.repo.IsBlocklisted(s.ctx, id)
	s.Require().NoError(err)
	s.False(isBlocked)
}

func (s *NodeRepoTestSuite) TestAddProviderAndList() {
	key := []byte("provider-data")
	pk, err := security.GenerateKeyFromSeed([]byte("peer123"))
	s.Require().NoError(err)

	warpPrivKey := pk.(warpnet.WarpPrivateKey)
	id, err := warpnet.IDFromPrivateKey(warpPrivKey)
	s.Require().NoError(err)

	provider := warpnet.PeerAddrInfo{ID: id}

	err = s.repo.AddProvider(s.ctx, key, provider)
	s.Require().NoError(err)

	providers, err := s.repo.GetProviders(s.ctx, key)
	s.Require().NoError(err)
	s.Len(providers, 1)

	all, err := s.repo.ListProviders()
	s.Require().NoError(err)
	s.NotEmpty(all)
}

func (s *NodeRepoTestSuite) TestAddAndRemoveInfo() {
	pk, err := security.GenerateKeyFromSeed([]byte("peerXYZ"))
	s.Require().NoError(err)

	warpPrivKey := pk.(warpnet.WarpPrivateKey)
	id, err := warpnet.IDFromPrivateKey(warpPrivKey)
	s.Require().NoError(err)

	info := warpnet.NodeInfo{
		ID: id,
	}

	err = s.repo.AddInfo(s.ctx, id, info)
	s.Require().NoError(err)

	err = s.repo.RemoveInfo(s.ctx, id)
	s.Require().NoError(err)
}

func (s *NodeRepoTestSuite) TestQuerySimple() {
	key := datastore.NewKey("query/key")
	val := []byte("qval")
	err := s.repo.Put(s.ctx, key, val)
	s.Require().NoError(err)

	q := query.Query{Prefix: "query/key"}
	results, err := s.repo.Query(s.ctx, q)
	s.Require().NoError(err)
	s.Require().NotNil(results)

	defer results.Close()
	var found bool
	for r := range results.Next() {
		if r.Error != nil {
			continue
		}
		found = true
		break
	}
	s.True(found)
}

func TestNodeRepoTestSuite(t *testing.T) {
	defer goleak.VerifyNone(t)

	suite.Run(t, new(NodeRepoTestSuite))
	closeWriter()
}
