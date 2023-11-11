// Copyright 2023 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import bolt "go.etcd.io/bbolt"

// newBoltDB creates a new bolt db.
func newBoltDB(file string) (boltDB, error) {
	db, err := bolt.Open(file, 0600, nil)
	if err != nil {
		return nil, err
	}
	db.NoSync = true
	db.NoFreelistSync = true
	db.FreelistType = bolt.FreelistMapType
	return (*wdb)(db), nil
}

// unwrapDBFn is a helper to transform func(boltTx) error into
// func(*bbolt.Tx) error.
func unwrapDBFn(fn func(boltTx) error) func(*bolt.Tx) error {
	return func(tx *bolt.Tx) error {
		return fn((*wtx)(tx))
	}
}

var _ boltDB = &wdb{}

type wdb bolt.DB

// Begin implements boltDB.
func (w *wdb) Begin(writable bool) (boltTx, error) {
	tx, err := (*bolt.DB)(w).Begin(writable)
	if err != nil {
		return nil, err
	}
	return (*wtx)(tx), nil
}

// View implements boltDB.
func (w *wdb) View(fn func(boltTx) error) error {
	return (*bolt.DB)(w).View(unwrapDBFn(fn))
}

// Batch implements boltDB.
func (w *wdb) Batch(fn func(boltTx) error) error {
	return (*bolt.DB)(w).Batch(unwrapDBFn(fn))
}

// Update implements boltDB.
func (w *wdb) Update(fn func(boltTx) error) error {
	return (*bolt.DB)(w).Update(unwrapDBFn(fn))
}

// Close implements boltDB.
func (w *wdb) Close() error {
	return (*bolt.DB)(w).Close()
}

var _ boltTx = &wtx{}

type wtx bolt.Tx

// Commit implements boltTx.
func (w *wtx) Commit() error {
	return (*bolt.Tx)(w).Commit()
}

// Rollback implements boltTx.
func (w *wtx) Rollback() error {
	return (*bolt.Tx)(w).Rollback()
}

// Bucket implements boltTx.
func (w *wtx) Bucket(name []byte) boltBucket {
	b := (*bolt.Tx)(w).Bucket(name)
	if b == nil {
		return nil
	}
	return b
}

// CreateBucketIfNotExists implements boltTx.
func (w *wtx) CreateBucketIfNotExists(name []byte) (boltBucket, error) {
	b, err := (*bolt.Tx)(w).CreateBucketIfNotExists(name)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// boltDB wraps bbolt.DB
type boltDB interface {
	Begin(writable bool) (boltTx, error)
	View(func(boltTx) error) error
	Batch(func(boltTx) error) error
	Update(func(boltTx) error) error
	Close() error
}

// boltTx wraps bbolt.Tx
type boltTx interface {
	Commit() error
	Rollback() error
	Bucket([]byte) boltBucket
	CreateBucketIfNotExists([]byte) (boltBucket, error)
}

// boltBucket wraps bbolt.Bucket
type boltBucket interface {
	Get(key []byte) []byte
	Put(key []byte, value []byte) error
}
