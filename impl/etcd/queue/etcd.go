// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package queue

import (
	"bytes"
	"encoding/gob"

	"github.com/google/key-transparency/core/queue"

	"golang.org/x/net/context"

	v3 "github.com/coreos/etcd/clientv3"
	recipe "github.com/coreos/etcd/contrib/recipes"
)

// Queue is a single-reader, multi-writer distributed queue.
type Queue struct {
	client *v3.Client
	ctx    context.Context

	keyPrefix string
}

type kv struct {
	Key          []byte
	Val          []byte
	AdvanceEpoch bool
}

// New creates a new consistent, distributed queue.
func New(client *v3.Client, mapID string) *Queue {
	return &Queue{client, context.TODO(), mapID}
}

// AdvanceEpoch submits an advance epoch request into the queue.
func (q *Queue) AdvanceEpoch() error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(kv{nil, nil, true}); err != nil {
		return err
	}
	return q.enqueue(buf.Bytes())
}

// Enqueue submits a key, value pair into the queue.
func (q *Queue) Enqueue(key, value []byte) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(kv{key, value, false}); err != nil {
		return err
	}
	return q.enqueue(buf.Bytes())
}

func (q *Queue) enqueue(val []byte) error {
	_, err := recipe.NewUniqueKV(q.client, q.keyPrefix, string(val), 0)
	return err
}

// Dequeue returns Enqueue()'d elements in FIFO order. If the
// queue is empty, Dequeue blocks until elements are available.
func (q *Queue) Dequeue(processFunc queue.ProcessKeyValueFunc, advanceFunc queue.AdvanceEpochFunc) error {
	return q.dequeue(func(data []byte) error {
		var dataKV kv
		dec := gob.NewDecoder(bytes.NewBuffer(data))
		if err := dec.Decode(&dataKV); err != nil {
			return err
		}

		if dataKV.AdvanceEpoch {
			return advanceFunc()
		}
		return processFunc(dataKV.Key, dataKV.Val)
	})
}