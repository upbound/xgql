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

import (
	"context"
	"fmt"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"go.etcd.io/bbolt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WithMockBoltDB(db *MockBoltDB) Option {
	return func(c *BBoltCache) {
		c.db = db
	}
}

func WithMockCache(ca *MockCache) Option {
	return func(c *BBoltCache) {
		c.Cache = ca
	}
}

func WithMarshalFn(fn MarshalFn) Option {
	return func(c *BBoltCache) {
		c.marshalFn = fn
	}
}

func WithUnmarshalFn(fn UnmarshalFn) Option {
	return func(c *BBoltCache) {
		c.unmarshalFn = fn
	}
}

var _ cache.Cache = &MockCache{}

type MockCache struct {
	MockGet   func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	MockList  func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	MockStart func(ctx context.Context) error
}

// GetInformer implements cache.Cache.
func (*MockCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	panic("unimplemented")
}

// GetInformerForKind implements cache.Cache.
func (*MockCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	panic("unimplemented")
}

// RemoveInformer implements cache.Cache.
func (*MockCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	panic("unimplemented")
}

// IndexField implements cache.Cache.
func (*MockCache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	panic("unimplemented")
}

// Start implements cache.Cache.
func (m *MockCache) Start(ctx context.Context) error {
	return m.MockStart(ctx)
}

// WaitForCacheSync implements cache.Cache.
func (*MockCache) WaitForCacheSync(ctx context.Context) bool {
	panic("unimplemented")
}

func (m *MockCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return m.MockGet(ctx, key, obj, opts...)
}

func (m *MockCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return m.MockList(ctx, list, opts...)
}

var _ boltDB = &MockBoltDB{}

type MockBoltDB struct {
	MockBegin  func(writable bool) (boltTx, error)
	MockView   func(fn func(tx boltTx) error) error
	MockBatch  func(fn func(tx boltTx) error) error
	MockUpdate func(fn func(tx boltTx) error) error
	MockClose  func() error
}

// Begin implements boltDB.
func (m *MockBoltDB) Begin(writable bool) (boltTx, error) {
	return m.MockBegin(writable)
}

func (m *MockBoltDB) View(fn func(tx boltTx) error) error {
	return m.MockView(fn)
}

func (m *MockBoltDB) Batch(fn func(tx boltTx) error) error {
	return m.MockBatch(fn)
}

func (m *MockBoltDB) Update(fn func(tx boltTx) error) error {
	return m.MockUpdate(fn)
}

func (m *MockBoltDB) Close() error {
	return m.MockClose()
}

func (m *MockBoltDB) GetMaxBatchSize() int {
	return bbolt.DefaultMaxBatchSize
}

var _ boltTx = &MockBoltTx{}

type MockBoltTx struct {
	MockCommit                  func() error
	MockRollback                func() error
	MockCreateBucketIfNotExists func([]byte) (boltBucket, error)
	MockBucket                  func([]byte) boltBucket
}

// Commit implements boltTx.
func (m *MockBoltTx) Commit() error {
	return m.MockCommit()
}

// Rollback implements boltTx.
func (m *MockBoltTx) Rollback() error {
	return m.MockRollback()
}

func (m *MockBoltTx) CreateBucketIfNotExists(bucket []byte) (boltBucket, error) {
	return m.MockCreateBucketIfNotExists(bucket)
}

func (m *MockBoltTx) Bucket(bucket []byte) boltBucket {
	return m.MockBucket(bucket)
}

var _ boltBucket = &MockBoltBucket{}

type MockBoltBucket struct {
	MockGet func(key []byte) []byte
	MockPut func(key, value []byte) error
}

func (m *MockBoltBucket) Get(key []byte) []byte {
	return m.MockGet(key)
}

func (m *MockBoltBucket) Put(key []byte, value []byte) error {
	return m.MockPut(key, value)
}

func TestCache_Get(t *testing.T) {
	errBoom := errors.New("boom")
	testKind := "ConfigMap"
	testAPIVersion := "v1"
	testTypeMeta := metav1.TypeMeta{
		Kind:       testKind,
		APIVersion: testAPIVersion,
	}
	testName := "test"
	testNamespace := "test-namespace"
	testUID := uuid.NewUUID()
	testObjectMeta := metav1.ObjectMeta{
		Name:      testName,
		Namespace: testNamespace,
		UID:       testUID,
	}
	testObjectBytes := []byte(fmt.Sprintf(`{"kind":"ConfigMap","apiVersion":"v1","metadata":{"name":%q,"namespace":%q}}`, testName, testNamespace))

	type args struct {
		key  client.ObjectKey
		obj  client.Object
		opts []client.GetOption
	}
	type want struct {
		err error
		obj client.Object
	}
	tests := map[string]struct {
		reason string
		cache  cache.Cache
		opts   []Option
		before func(*BBoltCache)
		after  func(*BBoltCache)
		args   args
		want   want
	}{
		"Success": {
			reason: "Should get object from bolt cache.",
			cache: &MockCache{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.ConfigMap); !ok {
						t.Errorf("\nc.cache.Get(...): want *corev1.ConfigMap, got %+#v", obj)
					}
					obj.SetUID(testUID)
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(b bool) (boltTx, error) {
						if diff := cmp.Diff(false, b); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								if diff := cmp.Diff("objects", string(b)); diff != "" {
									t.Errorf("\ntx.Bucket(...): -want bucket, +got bucket:\n%s", diff)
								}
								return &MockBoltBucket{
									MockGet: func(k []byte) []byte {
										if diff := cmp.Diff(string(testUID), string(k)); diff != "" {
											t.Errorf("\nb.Get(...): -want key, +got key:\n%s", diff)
										}
										return testObjectBytes
									},
								}
							},
							MockRollback: func() error { return nil },
						}, nil
					},
				}),
			},
			args: args{
				obj: &corev1.ConfigMap{},
				key: client.ObjectKey{
					Name:      testName,
					Namespace: testNamespace,
				},
			},
			want: want{
				obj: &corev1.ConfigMap{
					TypeMeta:   testTypeMeta,
					ObjectMeta: testObjectMeta,
				},
			},
		},
		"CacheError": {
			reason: "Should return the error unmodified.",
			cache: &MockCache{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.ConfigMap); !ok {
						t.Errorf("\nc.cache.Get(...): want *corev1.ConfigMap, got %+#v", obj)
					}
					return errBoom
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{}),
			},
			args: args{
				obj: &corev1.ConfigMap{},
				key: client.ObjectKey{
					Name:      testName,
					Namespace: testNamespace,
				},
			},
			want: want{
				err: errBoom,
				obj: &corev1.ConfigMap{},
			},
		},
		"TxBeginError": {
			reason: "Should return error if cannot begin read transaction.",
			cache: &MockCache{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.ConfigMap); !ok {
						t.Errorf("\nc.cache.Get(...): want *corev1.ConfigMap, got %+#v", obj)
					}
					obj.SetUID(testUID)
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(b bool) (boltTx, error) {
						if diff := cmp.Diff(false, b); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return nil, errBoom
					},
				}),
			},
			args: args{
				obj: &corev1.ConfigMap{},
				key: client.ObjectKey{
					Name:      testName,
					Namespace: testNamespace,
				},
			},
			want: want{
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
				},
				err: errors.Wrap(errors.Wrap(errBoom, errTxBegin), errObjectGet),
			},
		},
		"TxRollbackError": {
			reason: "Should return error if cannot rollback read transaction.",
			cache: &MockCache{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.ConfigMap); !ok {
						t.Errorf("\nc.cache.Get(...): want *corev1.ConfigMap, got %+#v", obj)
					}
					obj.SetUID(testUID)
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(b bool) (boltTx, error) {
						if diff := cmp.Diff(false, b); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								if diff := cmp.Diff("objects", string(b)); diff != "" {
									t.Errorf("\ntx.Bucket(...): -want bucket, +got bucket:\n%s", diff)
								}
								return &MockBoltBucket{
									MockGet: func(k []byte) []byte {
										if diff := cmp.Diff(string(testUID), string(k)); diff != "" {
											t.Errorf("\nb.Get(...): -want key, +got key:\n%s", diff)
										}
										return testObjectBytes
									},
								}
							},
							MockRollback: func() error { return errBoom },
						}, nil
					},
				}),
			},
			args: args{
				obj: &corev1.ConfigMap{},
				key: client.ObjectKey{
					Name:      testName,
					Namespace: testNamespace,
				},
			},
			want: want{
				obj: &corev1.ConfigMap{
					TypeMeta:   testTypeMeta,
					ObjectMeta: testObjectMeta,
				},
				err: errors.Wrap(errors.Wrap(errBoom, errTxRollback), errObjectGet),
			},
		},
		"NoBucket": {
			reason: "Should return error if bucket does not exist.",
			cache: &MockCache{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(b bool) (boltTx, error) {
						if diff := cmp.Diff(false, b); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								return nil
							},
							MockRollback: func() error { return nil },
						}, nil
					},
				}),
			},
			args: args{
				obj: &corev1.ConfigMap{},
				key: client.ObjectKey{
					Name:      testName,
					Namespace: testNamespace,
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtNoBucket, "objects"), errObjectGet),
				obj: &corev1.ConfigMap{},
			},
		},
		"NoKey": {
			reason: "Should return error if key does not exist.",
			cache: &MockCache{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.ConfigMap); !ok {
						t.Errorf("\nc.cache.Get(...): want *corev1.ConfigMap, got %+#v", obj)
					}
					obj.SetUID(testUID)
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(b bool) (boltTx, error) {
						if diff := cmp.Diff(false, b); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								return &MockBoltBucket{
									MockGet: func(k []byte) []byte {
										return nil
									},
								}
							},
							MockRollback: func() error { return nil },
						}, nil
					},
				}),
			},
			args: args{
				obj: &corev1.ConfigMap{},
				key: client.ObjectKey{
					Name:      testName,
					Namespace: testNamespace,
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtNoKey, testUID), errObjectGet),
				obj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
				},
			},
		},
		"UnmarshalError": {
			reason: "Should return error if object cannot be unmarshalled.",
			cache: &MockCache{
				MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*corev1.ConfigMap); !ok {
						t.Errorf("\nc.cache.Get(...): want *corev1.ConfigMap, got %+#v", obj)
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(b bool) (boltTx, error) {
						if diff := cmp.Diff(false, b); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								return &MockBoltBucket{
									MockGet: func(b []byte) []byte {
										return []byte("invalid")
									},
								}
							},
							MockRollback: func() error { return nil },
						}, nil
					},
				}),
				WithUnmarshalFn(func([]byte, any) error {
					return errBoom
				}),
			},
			args: args{
				obj: &corev1.ConfigMap{},
				key: client.ObjectKey{
					Name:      testName,
					Namespace: testNamespace,
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errObjectUnmarshal), errObjectGet),
				obj: &corev1.ConfigMap{},
			},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := NewBBoltCache(tc.cache, scheme.Scheme, "", tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if tc.before != nil {
				tc.before(c)
			}
			err = c.Get(context.TODO(), tc.args.key, tc.args.obj, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nc.Get(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.obj, tc.args.obj); diff != "" {
				t.Errorf("\n%s\nc.Get(...): -want, +got:\n%s", tc.reason, diff)
			}
			if tc.after != nil {
				tc.after(c)
			}
		})
	}
}

func TestCache_List(t *testing.T) {
	errBoom := errors.New("boom")
	testAPIVersion := "v1"
	testKind := "ConfigMap"
	testTypeMeta := metav1.TypeMeta{
		Kind:       testKind,
		APIVersion: testAPIVersion,
	}
	testNamespace := "test-namespace"
	testNames := []string{"test1", "test2"}
	testUIDs := []types.UID{uuid.NewUUID(), uuid.NewUUID()}
	testObjectBytes := make(map[string][]byte, len(testNames))
	for i := range testNames {
		testObjectBytes[string(testUIDs[i])] = []byte(fmt.Sprintf(`{"kind":"ConfigMap","apiVersion":"v1","metadata":{"name":"%s","namespace":"%s"}}`, testNames[i], testNamespace))
	}
	type args struct {
		list client.ObjectList
		opts []client.ListOption
	}
	type want struct {
		err  error
		list client.ObjectList
	}
	tests := map[string]struct {
		reason string
		cache  cache.Cache
		opts   []Option
		before func(*BBoltCache)
		after  func(*BBoltCache)
		args   args
		want   want
	}{
		"Success": {
			reason: "Should successfully list objects from the cache.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					l, ok := list.(*corev1.ConfigMapList)
					if !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
						return nil
					}
					l.Items = []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(writable bool) (boltTx, error) {
						if diff := cmp.Diff(false, writable); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								if diff := cmp.Diff("objects", string(b)); diff != "" {
									t.Errorf("\ntx.Bucket(...): -want bucket, +got bucket:\n%s", diff)
									return nil
								}
								return &MockBoltBucket{
									MockGet: func(b []byte) []byte {
										return testObjectBytes[string(b)]
									},
								}
							},
							MockRollback: func() error {
								return nil
							},
						}, nil
					},
				}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				list: &corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						{
							TypeMeta: testTypeMeta,
							ObjectMeta: metav1.ObjectMeta{
								Name:      testNames[0],
								Namespace: testNamespace,
								UID:       testUIDs[0],
							},
						},
						{
							TypeMeta: testTypeMeta,
							ObjectMeta: metav1.ObjectMeta{
								Name:      testNames[1],
								Namespace: testNamespace,
								UID:       testUIDs[1],
							},
						},
					},
				},
			},
		},
		"EmptyList": {
			reason: "Should skip listing objects from cache.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*corev1.ConfigMapList); !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
						return nil
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(bool) (boltTx, error) {
						t.Errorf("\nc.List(...): should not call tx.Bucket(...)")
						return nil, errBoom
					},
				}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				list: &corev1.ConfigMapList{},
			},
		},
		"CacheError": {
			reason: "Should return error unmodified.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					_, ok := list.(*corev1.ConfigMapList)
					if !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
					}
					return errBoom
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				list: &corev1.ConfigMapList{},
				err:  errBoom,
			},
		},
		"TxBeginError": {
			reason: "Should return an error if cannot begin read transactions.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					l, ok := list.(*corev1.ConfigMapList)
					if !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
						return nil
					}
					l.Items = []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(writable bool) (boltTx, error) {
						if diff := cmp.Diff(false, writable); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return nil, errBoom
					},
				}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errTxBegin), errObjectList),
				list: &corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					},
				},
			},
		},
		"NoBucket": {
			reason: "Should return an error if the bucket does not exist.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					l, ok := list.(*corev1.ConfigMapList)
					if !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
						return nil
					}
					l.Items = []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(writable bool) (boltTx, error) {
						if diff := cmp.Diff(false, writable); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								return nil
							},
							MockRollback: func() error {
								return nil
							},
						}, nil
					},
				}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtNoBucket, "objects"), errObjectList),
				list: &corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					},
				},
			},
		},
		"NoKey": {
			reason: "Should skip if the key does not exist.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					l, ok := list.(*corev1.ConfigMapList)
					if !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
						return nil
					}
					l.Items = []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(writable bool) (boltTx, error) {
						if diff := cmp.Diff(false, writable); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								return &MockBoltBucket{
									MockGet: func(b []byte) []byte {
										return nil
									},
								}
							},
							MockRollback: func() error {
								return nil
							},
						}, nil
					},
				}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				list: &corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					},
				},
				err: errors.Wrap(errors.Errorf(errFmtNoKey, testUIDs[0]), errObjectList),
			},
		},
		"TxRollbackError": {
			reason: "Should return error if read tx cannot be rolled back.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					l, ok := list.(*corev1.ConfigMapList)
					if !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
						return nil
					}
					l.Items = []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(writable bool) (boltTx, error) {
						if diff := cmp.Diff(false, writable); diff != "" {
							t.Errorf("\ndb.Begin(...): -want writable, +got writable:\n%s", diff)
						}
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								return &MockBoltBucket{
									MockGet: func(b []byte) []byte {
										return testObjectBytes[string(b)]
									},
								}
							},
							MockRollback: func() error {
								return errBoom
							},
						}, nil
					},
				}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				list: &corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						{
							TypeMeta: testTypeMeta,
							ObjectMeta: metav1.ObjectMeta{
								Name:      testNames[0],
								Namespace: testNamespace,
								UID:       testUIDs[0],
							},
						},
						{
							TypeMeta: testTypeMeta,
							ObjectMeta: metav1.ObjectMeta{
								Name:      testNames[1],
								Namespace: testNamespace,
								UID:       testUIDs[1],
							},
						},
					},
				},
				err: errors.Wrap(errors.Wrap(errBoom, errTxRollback), errObjectList),
			},
		},
		"UnmarshalError": {
			reason: "Should return an error if the object cannot be unmarshalled.",
			cache: &MockCache{
				MockList: func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					l, ok := list.(*corev1.ConfigMapList)
					if !ok {
						t.Errorf("\nc.cache.List(...): want *corev1.ConfigMapList, got %+#v", list)
						return nil
					}
					l.Items = []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					}
					return nil
				},
			},
			opts: []Option{
				WithMockBoltDB(&MockBoltDB{
					MockBegin: func(writable bool) (boltTx, error) {
						return &MockBoltTx{
							MockBucket: func(b []byte) boltBucket {
								return &MockBoltBucket{
									MockGet: func(b []byte) []byte {
										return []byte("invalid")
									},
								}
							},
							MockRollback: func() error {
								return nil
							},
						}, nil
					},
				}),
				WithUnmarshalFn(func([]byte, any) error {
					return errBoom
				}),
			},
			args: args{
				list: &corev1.ConfigMapList{},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errObjectUnmarshal), errObjectList),
				list: &corev1.ConfigMapList{
					Items: []corev1.ConfigMap{
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[0],
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								UID: testUIDs[1],
							},
						},
					},
				},
			},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := NewBBoltCache(tc.cache, scheme.Scheme, "", tc.opts...)
			c.deepCopy = true
			if err != nil {
				t.Fatal(err)
			}
			if tc.before != nil {
				tc.before(c)
			}
			err = c.List(context.TODO(), tc.args.list, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nc.List(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.list, tc.args.list); diff != "" {
				t.Errorf("\n%s\nc.List(...): -want, +got:\n%s", tc.reason, diff)
			}
			if tc.after != nil {
				tc.after(c)
			}
		})
	}
}
