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

// Package events implements stored event log used for audit and other
// purposes
package events

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/session"

	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
)

const (
	// DefaultLimit is a default limit set for event queries
	DefaultLimit = 20
	// MaxLimit is a maximum limit set for event queries
	MaxLimit = 100
	// Asc is ascending sort order
	Asc = 1
	// Desc is descending sort order
	Desc = -1
)

// Filter is event search filter
type Filter struct {
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Limit     int       `json:"limit"`
	Order     int       `json:"order"`
	SessionID string    `json:"session_id"`
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

// Log is an event logger interface
type Log interface {
	Log(id lunk.EventID, e lunk.Event)
	LogEntry(lunk.Entry) error
	LogSession(session.Session) error
	GetEvents(filter Filter) ([]lunk.Entry, error)
	GetSessionEvents(filter Filter) ([]session.Session, error)
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
			return nil, trace.Wrap(teleport.BadParameter("start", "need start in RFC3339 format"))
		}
	}
	if vals.Get("end") != "" {
		if err = f.End.UnmarshalText([]byte(vals.Get("end"))); err != nil {
			return nil, trace.Wrap(teleport.BadParameter("end", "need end in RFC3339 format"))
		}
	}

	if vals.Get("limit") != "" {
		if f.Limit, err = strconv.Atoi(vals.Get("limit")); err != nil {
			return nil, trace.Wrap(teleport.BadParameter("limit", "limits need to be int"))
		}
	}

	if vals.Get("order") != "" {
		if f.Order, err = strconv.Atoi(vals.Get("order")); err != nil {
			return nil, trace.Wrap(teleport.BadParameter("limit", "order is 1 for Ascending, -1 for descending"))
		}
	}
	f.SessionID = vals.Get("sid")
	return &f, nil
}
