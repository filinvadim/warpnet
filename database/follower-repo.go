package database

import (
	"github.com/filinvadim/dWighter/database/storage"
)

const FollowRepoName = "FOLLOW"

// FollowRepo handles reader/writer relationships
type FollowRepo struct {
	db *storage.DB
}

func NewFollowRepo(db *storage.DB) *FollowRepo {
	return &FollowRepo{db: db}
}

//// Follow adds a reader-writer relationship in both directions
//func (repo *FollowRepo) Follow(readerID, writerID string) error {
//	return repo.db.Txn(func(txn *badger.Txn) error {
//		// Save the relationship as "reader:user1:writer:user2"
//		readerKey, err := storage.NewPrefixBuilder(FollowRepoName).AddReaderId(readerID).AddWriterId(writerID).Build()
//		if err != nil {
//			return err
//		}
//
//		// Save the relationship as "writer:user2:reader:user1"
//		writerKey, err := storage.NewPrefixBuilder(FollowRepoName).AddWriterId(writerID).AddReaderId(readerID).Build()
//		if err != nil {
//			return err
//		}
//
//		// Save both keys in the database within the transaction
//		if err := txn.Set([]byte(readerKey), []byte("1")); err != nil {
//			return err
//		}
//
//		return txn.Set([]byte(writerKey), []byte("1"))
//	})
//}
//
//// Unfollow removes a reader-writer relationship in both directions
//func (repo *FollowRepo) Unfollow(readerID, writerID string) error {
//	return repo.db.Txn(func(txn *badger.Txn) error {
//		// Remove the relationship as "reader:user1:writer:user2"
//		readerKey, err := storage.NewPrefixBuilder(FollowRepoName).AddReaderId(readerID).AddWriterId(writerID).Build()
//		if err != nil {
//			return err
//		}
//
//		// Remove the relationship as "writer:user2:reader:user1"
//		writerKey, err := storage.NewPrefixBuilder(FollowRepoName).AddWriterId(writerID).AddReaderId(readerID).Build()
//		if err != nil {
//			return err
//		}
//
//		// Remove both keys within the transaction
//		if err := txn.Delete([]byte(readerKey)); err != nil {
//			return err
//		}
//
//		return txn.Delete([]byte(writerKey))
//	})
//}
//
//// GetReaders retrieves all readers of a given writer
//func (repo *FollowRepo) GetReaders(writerID string) ([]string, error) {
//	prefix, err := storage.NewPrefixBuilder(FollowRepoName).AddWriterId(writerID).Build()
//	if err != nil {
//		return nil, err
//	}
//
//	readers := make([]string, 0)
//	err = repo.db.IterateKeys(prefix, func(key string) error {
//		// FOLLOW:writer:c09f21be-22bf-4ab6-9450-7db8c822a526:reader:
//		prefixForReader, err := storage.NewPrefixBuilder(prefix.String()).AddReaderId("").Build()
//		if err != nil {
//			return err
//		}
//		readerID := strings.TrimPrefix(key, prefixForReader.String())
//		readers = append(readers, readerID)
//		return nil
//	})
//	if err != nil {
//		return nil, err
//	}
//	return readers, nil
//}
//
//// GetWriters retrieves all writers a given reader follows
//func (repo *FollowRepo) GetWriters(readerID string) ([]string, error) {
//	prefix, err := storage.NewPrefixBuilder(FollowRepoName).AddReaderId(readerID).Build()
//	if err != nil {
//		return nil, err
//	}
//
//	writers := make([]string, 0)
//	err = repo.db.IterateKeys(prefix, func(key string) error {
//		prefixForWriter, err := storage.NewPrefixBuilder(prefix.String()).AddWriterId("").Build()
//		if err != nil {
//			return err
//		}
//
//		// FOLLOW:reader:c09f21be-22bf-4ab6-9450-7db8c822a526:writer:
//		writerID := strings.TrimPrefix(key, prefixForWriter.String())
//		writers = append(writers, writerID)
//		return nil
//	})
//	if err != nil {
//		return nil, err
//	}
//	return writers, nil
//}
