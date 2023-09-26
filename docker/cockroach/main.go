package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

type changeFeed struct {
	Table string `json:"table"`
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

type Event struct {
	// Type is operation type
	Type types.OpType
	// Item is event Item
	Item Item
}

// Item is a key value item
type Item struct {
	// Key is a key of the key value item
	Key []byte
	// Value is a value of the key value item
	Value []byte
	// Expires is an optional record expiry time
	Expires time.Time
	// ID is a record ID, newer records have newer ids
	ID int64
	// LeaseID is a lease ID, could be set on objects
	// with TTL
	LeaseID int64
}

func (cf changeFeed) events() ([]Event, error) {
	var keys []HexData
	if err := json.Unmarshal(cf.Key, &keys); err != nil {
		return nil, trace.Wrap(err, "unable to unmarshal key")
	}
	fmt.Println("key: ", string(keys[0]))
	e := Event{
		Item: Item{
			Key: []byte(keys[0]),
		},
	}
	c := change{}
	if err := json.Unmarshal(cf.Value, &c); err != nil {
		return nil, trace.Wrap(err, "unable to unmarshal value")
	}

	if c.After == nil {
		e.Type = types.OpDelete
		return []Event{e}, nil
	}
	e.Type = types.OpPut
	item, err := c.After.toItem()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e.Item = item
	if c.Before != nil && !reflect.DeepEqual(c.After.Key, c.Before.Key) {
		item, err := c.Before.toItem()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return []Event{{
			Type: types.OpDelete,
			Item: item,
		}, e}, nil
	}

	return []Event{e}, nil
}

type change struct {
	Before *kv `json:"before"`
	After  *kv `json:"after"`
}

type kv struct {
	Key      HexData `json:"key"`
	Value    HexData `json:"value"`
	Expires  *string `json:"expires"`
	Revision string  `json:"revision"`
}

type HexData []byte

func (h *HexData) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.TrimPrefix(s, "\\x")
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	*h = HexData(decoded)
	return nil
}

func (kv *kv) toItem() (Item, error) {
	item := Item{
		Key:   kv.Key,
		Value: kv.Value,
	}

	if kv.Expires != nil {
		t, err := time.Parse("2006-01-02T15:04:05Z", *kv.Expires)
		if err != nil {
			return item, trace.Wrap(err)
		}
		item.Expires = t
	}
	var err error
	item.ID, err = idFromRevision(kv.Revision)
	if err != nil {
		return item, trace.Wrap(err)
	}
	return item, nil
}

func idFromRevision(rev string) (int64, error) {
	uuid, err := uuid.Parse(rev)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	u := binary.LittleEndian.Uint64(uuid[:])
	u &= 0x7fff_ffff_ffff_ffff
	return int64(u), nil
}

func run() error {
	ctx := context.Background()
	pgConn, err := pgx.Connect(ctx, "postgresql://root@localhost:26257/teleport_backend?sslmode=verify-full&sslrootcert=certs/ca.crt&sslcert=certs/root.crt&sslkey=certs/root.key&sslmode=verify-full")
	if err != nil {
		return err
	}
	defer pgConn.Close(ctx)

	rows, err := pgConn.Query(ctx, "EXPERIMENTAL CHANGEFEED FOR kv WITH diff, no_initial_scan;")
	if err != nil {
		return err
	}

	cf := &changeFeed{}
	for rows.Next() {
		err := rows.Scan(&cf.Table, &cf.Key, &cf.Value)
		if err != nil {
			return err
		}
		events, err := cf.events()
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(events)
	}
	return nil
}
