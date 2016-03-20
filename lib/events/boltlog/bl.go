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
	"encoding/binary"
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
	sessionBytes, err := json.Marshal(sess)
	if err != nil {
		return trace.Wrap(err)
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		bkt, err := boltbk.UpsertBucket(tx, []string{"sessionlog"})
		if err != nil {
			return trace.Wrap(err)
		}
		if err := bkt.Put(sess.ID.UUID(), sessionBytes); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

func (b *BoltLog) GetSessionEvents(f events.Filter) ([]session.Session, error) {
	if f.SessionID != "" {
		if err := f.SessionID.Check(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if f.Order != events.Desc {
		return nil, trace.Wrap(teleport.BadParameter("order", "only descending order is supported with this backend"))
	}
	if f.Start.IsZero() {
		return nil, trace.Wrap(teleport.BadParameter("start", "supply starting point"))
	}
	if f.Limit > events.MaxLimit {
		return nil, trace.Wrap(teleport.BadParameter("limit", "limit exceeds maximum"))
	}
	if f.Limit == 0 {
		f.Limit = events.DefaultLimit
	}
	startKey := maxTimeUUID(f.Start)
	if f.SessionID != "" {
		startKey = f.SessionID.UUID()
	}
	endKey := minTimeUUID(f.End)

	out := []session.Session{}
	err := b.db.View(func(tx *bolt.Tx) error {
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
			bkt, err = boltbk.GetBucket(tx, []string{"sessions", string(f.SessionID)})
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

// fromTimeAndBits composes fake UUID based on given time
// and most significant bits
func fromTimeAndBits(tm time.Time, highBits uint64) []byte {
	bytes := make([]byte, 16)

	utcTime := tm.In(time.UTC)

	// according to https://tools.ietf.org/html/rfc4122#page-8 time portion is
	// count of 100 nanosecond intervals since 00:00:00.00, 15 October 1582 (the date of
	// Gregorian reform to the Christian calendar).
	count := uint64(utcTime.Unix()-timeBase)*10000000 + uint64(utcTime.Nanosecond()/100)

	low := uint32(count & 0xffffffff)
	mid := uint16((count >> 32) & 0xffff)
	hi := uint16((count >> 48) & 0x0fff)
	hi |= 0x1000 // Version 1

	binary.BigEndian.PutUint32(bytes[0:], low)
	binary.BigEndian.PutUint16(bytes[4:], mid)
	binary.BigEndian.PutUint16(bytes[6:], hi)
	binary.BigEndian.PutUint64(bytes[8:], highBits)

	return bytes
}

// the date of Gregorian reform to the Christian calendar
var timeBase = time.Date(1582, time.October, 15, 0, 0, 0, 0, time.UTC).Unix()

func maxTimeUUID(tm time.Time) []byte {
	return fromTimeAndBits(tm, maxClockSeqAndNode)
}

func minTimeUUID(tm time.Time) []byte {
	return fromTimeAndBits(tm, minClockSeqAndNode)
}

const (
	minClockSeqAndNode uint64 = 0x8080808080808080
	maxClockSeqAndNode uint64 = 0x7f7f7f7f7f7f7f7f
)
