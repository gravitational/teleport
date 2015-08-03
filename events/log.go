package events

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
)

type Filter struct {
	Start     time.Time
	End       time.Time
	Limit     int
	Order     int
	SessionID string
}

func (f Filter) String() string {
	st, err := f.Start.MarshalText()
	if err != nil {
		return err.Error()
	}
	if f.Start.IsZero() {
		st = []byte("<zero-time>")
	}
	et, err := f.End.MarshalText()
	if err != nil {
		return err.Error()
	}
	if f.End.IsZero() {
		et = []byte("<zero-time>")
	}
	return fmt.Sprintf(
		"Filter(start=%v, end=%v, limit=%v, order=%v, sid=%v)",
		string(st), string(et), f.Limit, f.Order, f.SessionID)
}

const (
	DefaultLimit = 20
	Asc          = 1
	Desc         = -1
)

type Log interface {
	Log(id lunk.EventID, e lunk.Event)
	LogEntry(lunk.Entry) error
	GetEvents(filter Filter) ([]lunk.Entry, error)
}

func FilterToURL(f Filter) (url.Values, error) {
	st, err := f.Start.MarshalText()
	if err != nil {
		return nil, err
	}
	et, err := f.End.MarshalText()
	if err != nil {
		return nil, err
	}
	vals := make(url.Values)
	vals.Set("start", string(st))
	vals.Set("end", string(et))
	vals.Set("limit", strconv.Itoa(f.Limit))
	vals.Set("order", strconv.Itoa(f.Order))
	vals.Set("sid", f.SessionID)
	return vals, nil
}

func FilterFromURL(vals url.Values) (*Filter, error) {
	var f Filter
	var err error
	if vals.Get("start") != "" {
		if err = f.Start.UnmarshalText([]byte(vals.Get("start"))); err != nil {
			return nil, err
		}
	}
	if vals.Get("end") != "" {
		if err = f.End.UnmarshalText([]byte(vals.Get("end"))); err != nil {
			return nil, err
		}
	}

	if vals.Get("limit") != "" {
		if f.Limit, err = strconv.Atoi(vals.Get("limit")); err != nil {
			return nil, err
		}
	}

	if vals.Get("order") != "" {
		if f.Order, err = strconv.Atoi(vals.Get("order")); err != nil {
			return nil, err
		}
	}
	f.SessionID = vals.Get("sid")
	return &f, nil
}
