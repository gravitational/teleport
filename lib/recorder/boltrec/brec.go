/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package boltrec implements boltdb-backed session recorder
package boltrec

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/recorder"

	log "github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/gravitational/trace"
)

func New(path string) (*boltRecorder, error) {
	br := boltRecorder{
		path: path,
		dbs:  make(map[string]*boltRW),
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert path")
	}
	if err := os.MkdirAll(path, 0777); err != nil {
		return nil, trace.Wrap(
			err, fmt.Sprintf("failed to create '%v' for session records", path))
	}
	// test if path is writeable
	testRef, err := br.getRef("testRecord")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = testRef.WriteChunks([]recorder.Chunk{recorder.Chunk{
		Data: []byte{1, 2, 3},
	}})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := br.decRef(testRef.rw); err != nil {
		return nil, trace.Wrap(err)
	}

	return &br, nil
}

type boltRecorder struct {
	sync.Mutex
	path string
	dbs  map[string]*boltRW
}

func (r *boltRecorder) Close() error {
	r.Lock()
	defer r.Unlock()

	for _, db := range r.dbs {
		if err := db.Close(); err != nil {
			log.Error(err)
		}
	}
	r.dbs = nil
	return nil
}

func (r *boltRecorder) decRef(b *boltRW) error {
	r.Lock()
	defer r.Unlock()

	b.refs -= 1
	if b.refs == 0 {
		delete(r.dbs, b.id)
		return b.Close()
	}
	return nil
}

func (r *boltRecorder) getRef(id string) (*boltRef, error) {
	r.Lock()
	defer r.Unlock()

	wr, ok := r.dbs[id]
	var err error
	if !ok {
		wr, err = newBoltRW(id, filepath.Join(r.path, id))
		if err != nil {
			return nil, err
		}
	} else {
		log.Infof("boltRecorder: getRef %v", id)
		wr.refs += 1
	}
	r.dbs[id] = wr
	return &boltRef{r: r, rw: wr}, nil
}

func (r *boltRecorder) GetChunkWriter(id string) (recorder.ChunkWriteCloser, error) {
	return r.getRef(id)
}

func (r *boltRecorder) GetChunkReader(id string) (recorder.ChunkReadCloser, error) {
	return r.getRef(id)
}

func newBoltRW(id, path string) (*boltRW, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	wr := &boltRW{
		id:   id,
		db:   db,
		refs: 1,
	}
	if err := wr.initWriteIter(); err != nil {
		return nil, err
	}
	return wr, nil
}

type boltRW struct {
	id   string
	db   *bolt.DB
	refs int
}

func (b *boltRW) GetChunksCount() (uint64, error) {
	var lastChunk uint64
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := boltbk.GetBucket(tx, []string{"iter"})
		if err != nil {
			return err
		}
		bytes := bkt.Get([]byte("val"))
		if bytes == nil {
			return trace.NotFound("not found")
		}
		lastChunk = binary.BigEndian.Uint64(bytes)
		return nil
	})
	return lastChunk, trace.Wrap(err)
}

func (b *boltRW) initWriteIter() error {
	var val []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bkt, err := boltbk.GetBucket(tx, []string{"iter"})
		if err != nil {
			return err
		}
		bytes := bkt.Get([]byte("val"))
		if bytes == nil {
			return trace.NotFound("not found")
		}
		val = make([]byte, len(bytes))
		copy(val, bytes)
		return nil
	})
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return err
	}
	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := boltbk.UpsertBucket(tx, []string{"iter"})
		if err != nil {
			return err
		}
		var bin = make([]byte, 8)
		binary.BigEndian.PutUint64(bin, 0)
		return bkt.Put([]byte("val"), bin)
	})
}

func (b *boltRW) WriteChunks(ch []recorder.Chunk) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		ibkt, err := boltbk.GetBucket(tx, []string{"iter"})
		if err != nil {
			return err
		}
		iterb := ibkt.Get([]byte("val"))
		if iterb == nil {
			return trace.NotFound("iter not found")
		}
		lastChunk := binary.BigEndian.Uint64(iterb)
		cbkt, err := boltbk.UpsertBucket(tx, []string{"chunks"})
		if err != nil {
			return err
		}
		bin := make([]byte, 8)
		for _, c := range ch {
			chunkb, err := json.Marshal(c)
			if err != nil {
				return err
			}
			lastChunk++
			binary.BigEndian.PutUint64(bin, lastChunk)
			if err := cbkt.Put(bin, chunkb); err != nil {
				return err
			}
		}
		return ibkt.Put([]byte("val"), bin)
	})
}

func (b *boltRW) ReadChunk(chunk uint64) ([]byte, error) {
	var bt []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bin := make([]byte, 8)
		binary.BigEndian.PutUint64(bin, chunk)
		cbkt, err := boltbk.GetBucket(tx, []string{"chunks"})
		if err != nil {
			return err
		}
		bytes := cbkt.Get(bin)
		if bytes == nil {
			return trace.NotFound("chunk not found")
		}
		bt = make([]byte, len(bytes))
		copy(bt, bytes)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return bt, nil
}

func (fw *boltRW) Close() error {
	return fw.db.Close()
}

type boltRef struct {
	rw *boltRW
	r  *boltRecorder
}

func (r *boltRef) Close() error {
	return r.r.decRef(r.rw)
}

func (r *boltRef) GetChunksCount() (uint64, error) {
	return r.rw.GetChunksCount()
}

func (r *boltRef) ReadChunks(start int, end int) ([]recorder.Chunk, error) {
	chunks := []recorder.Chunk{}
	for i := start; i < end; i++ {
		out, err := r.rw.ReadChunk(uint64(i))
		if err != nil {
			if trace.IsNotFound(err) {
				return chunks, nil
			}
			return nil, err
		}
		var ch *recorder.Chunk
		if err := json.Unmarshal(out, &ch); err != nil {
			return nil, err
		}
		chunks = append(chunks, *ch)
	}
	return chunks, nil
}

func (r *boltRef) WriteChunks(ch []recorder.Chunk) error {
	return r.rw.WriteChunks(ch)
}
