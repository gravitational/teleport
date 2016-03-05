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

// Package boltlog is a Bolt-backend implementation of the event log
package boltlog

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	"github.com/boltdb/bolt"
	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
)

type BoltLog struct {
	db *bolt.DB
}

func New(path string) (*BoltLog, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &BoltLog{
		db: db,
	}, nil
}

func (b *BoltLog) Close() error {
	return b.db.Close()
}

func (b *BoltLog) Log(id lunk.EventID, e lunk.Event) {
	en := lunk.NewEntry(id, e)
	en.Time = time.Now()
	b.LogEntry(en)
}

func (b *BoltLog) LogEntry(en lunk.Entry) error {
	sid := en.Properties[SessionID]
	btm, err := en.Time.MarshalText()
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(en)
	if err != nil {
		return err
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		if sid != "" {
			bkt, err := boltbk.UpsertBucket(tx, []string{"sessions", sid})
			if err != nil {
				return err
			}
			if err := bkt.Put([]byte(btm), bytes); err != nil {
				return err
			}
		}
		bkt, err := boltbk.UpsertBucket(tx, []string{"events"})
		if err != nil {
			return err
		}
		return bkt.Put([]byte(btm), bytes)
	})
}

func (b *BoltLog) LogSession(sess session.Session) error {
	lastActive, err := sess.LastActive.MarshalText()
	if err != nil {
		return trace.Wrap(err)
	}

	sessionBytes, err := json.Marshal(sess)
	if err != nil {
		return trace.Wrap(err)
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := boltbk.UpsertBucket(tx, []string{"sessionlog"})
		if err != nil {
			return trace.Wrap(err)
		}
		if err := bkt.Put([]byte(lastActive), sessionBytes); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

func (b *BoltLog) GetSessionEvents(f events.Filter) ([]session.Session, error) {
	if f.Start.IsZero() {
		return nil, trace.Wrap(teleport.BadParameter("start", "supply starting point"))
	}
	if f.Limit > events.MaxLimit {
		return nil, trace.Wrap(teleport.BadParameter("limit", "limit exceeds maximum"))
	}
	if f.Limit == 0 {
		f.Limit = events.DefaultLimit
	}
	startKey, err := f.Start.MarshalText()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endKey, err := f.End.MarshalText()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := []session.Session{}
	err = b.db.View(func(tx *bolt.Tx) error {
		bkt, err := boltbk.GetBucket(tx, []string{"sessionlog"})
		if err != nil {
			if teleport.IsNotFound(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		// choose start key and iter function based on start or end
		c := bkt.Cursor()
		var count = 0
		var flimit limitfn
		var fnext nextfn
		var key, val []byte
		if f.Order >= 0 { // ascending
			fnext = c.Next
			flimit = func() bool {
				if !f.End.IsZero() && bytes.Compare(key, endKey) > 0 {
					return false
				}
				return count < f.Limit
			}
		} else {
			fnext = c.Prev // descending
			flimit = func() bool {
				if !f.End.IsZero() && bytes.Compare(key, endKey) <= 0 {
					return false
				}
				return count < f.Limit
			}
		}
		key, val = c.Seek(startKey)
		// this can happen if we supplied the key after last key
		// we jump to last key to check if it still matches
		if key == nil && f.Order < 0 {
			key, val = c.Last()
		}

		for ; key != nil && flimit(); key, val = fnext() {
			var sess *session.Session
			if err := json.Unmarshal(val, &sess); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, *sess)
			count++
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func (b *BoltLog) GetEvents(f events.Filter) ([]lunk.Entry, error) {
	if f.Start.IsZero() {
		return nil, trace.Wrap(teleport.BadParameter("start", "supply starting point"))
	}
	if f.Limit == 0 {
		f.Limit = events.DefaultLimit
	}
	startKey, err := f.Start.MarshalText()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endKey, err := f.End.MarshalText()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := []lunk.Entry{}
	err = b.db.View(func(tx *bolt.Tx) error {
		var bkt *bolt.Bucket
		var err error
		if f.SessionID != "" {
			bkt, err = boltbk.GetBucket(tx, []string{"sessions", f.SessionID})
		} else {
			bkt, err = boltbk.GetBucket(tx, []string{"events"})
		}
		if err != nil {
			if teleport.IsNotFound(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		// choose start key and iter function based on start or end
		c := bkt.Cursor()
		var count = 0
		var flimit limitfn
		var fnext nextfn
		var key, val []byte
		if f.Order >= 0 { // ascending
			fnext = c.Next
			flimit = func() bool {
				if !f.End.IsZero() && bytes.Compare(key, endKey) > 0 {
					return false
				}
				return count < f.Limit
			}
		} else {
			fnext = c.Prev // descending
			flimit = func() bool {
				if !f.End.IsZero() && bytes.Compare(key, endKey) <= 0 {
					return false
				}
				return count < f.Limit
			}
		}
		key, val = c.Seek(startKey)
		// this can happen if we supplied the key after last key
		// we jump to last key to check if it still matches
		if key == nil && f.Order < 0 {
			key, val = c.Last()
		}
		for ; key != nil && flimit(); key, val = fnext() {
			var e *lunk.Entry
			if err := json.Unmarshal(val, &e); err != nil {
				return err
			}
			out = append(out, *e)
			count++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

const SessionID = "sessionid"

type nextfn func() ([]byte, []byte)
type limitfn func() bool
