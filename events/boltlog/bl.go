package boltlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/events"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/boltdb/bolt"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
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

func (b *BoltLog) GetEvents(f events.Filter) ([]lunk.Entry, error) {
	log.Infof("Get events: %v", f)
	if f.Start.IsZero() {
		return nil, fmt.Errorf("supply either starting point")
	}
	if f.Limit == 0 {
		f.Limit = events.DefaultLimit
	}

	out := []lunk.Entry{}
	err := b.db.View(func(tx *bolt.Tx) error {
		var bkt *bolt.Bucket
		var err error
		if f.SessionID != "" {
			bkt, err = boltbk.GetBucket(tx, []string{"sessions", f.SessionID})
		} else {
			bkt, err = boltbk.GetBucket(tx, []string{"events"})
		}
		if err != nil {
			if backend.IsNotFound(err) {
				return nil
			}
			return err
		}
		startKey, err := f.Start.MarshalText()
		if err != nil {
			return err
		}
		endKey, err := f.End.MarshalText()
		if err != nil {
			return err
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
			count += 1
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
