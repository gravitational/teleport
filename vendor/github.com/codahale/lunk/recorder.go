package lunk

import (
	"encoding/csv"
	"sort"
	"strconv"
	"time"
)

// An EntryRecorder records entries, e.g. to a streaming processing system or an
// OLAP database.
type EntryRecorder interface {
	Record(Entry) error
}

// NewNormalizedCSVEntryRecorder returns an EntryRecorder which writes events to
// one CSV file and properties to another.
func NewNormalizedCSVEntryRecorder(events, props *csv.Writer) EntryRecorder {
	return nCSVRecorder{
		events: events,
		props:  props,
	}
}

// NewDenormalizedCSVEntryRecorder returns an EntryRecorder which writes events
// and their properties to a single CSV file, duplicating event data when
// necessary.
func NewDenormalizedCSVEntryRecorder(w *csv.Writer) EntryRecorder {
	return dCSVRecorder{
		w: w,
	}
}

var (
	// NormalizedEventHeaders are the set of headers used for storing events in
	// normalized CSV files.
	NormalizedEventHeaders = []string{
		"root",
		"id",
		"parent",
		"schema",
		"time",
		"host",
		"pid",
		"deploy",
	}

	// NormalizedPropertyHeaders are the set of headers used for storing
	// properties in normalized CSV files.
	NormalizedPropertyHeaders = []string{
		"root",
		"id",
		"parent",
		"prop_name",
		"prop_value",
	}

	// DenormalizedEventHeaders are the set of headers used for storing events
	// in denormalized CSV files.
	DenormalizedEventHeaders = []string{
		"root",
		"id",
		"parent",
		"schema",
		"time",
		"host",
		"pid",
		"deploy",
		"prop_name",
		"prop_value",
	}
)

type nCSVRecorder struct {
	events *csv.Writer
	props  *csv.Writer
}

func (r nCSVRecorder) Record(e Entry) error {
	root, id, parent := e.Root.String(), e.ID.String(), e.Parent.String()

	if err := r.events.Write([]string{
		root,
		id,
		parent,
		e.Schema,
		e.Time.Format(time.RFC3339Nano),
		e.Host,
		strconv.Itoa(e.PID),
		e.Deploy,
	}); err != nil {
		return err
	}

	keys := make([]string, 0, len(e.Properties))
	for k := range e.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := e.Properties[k]
		if err := r.props.Write([]string{
			root,
			id,
			parent,
			k,
			v,
		}); err != nil {
			return err
		}

	}

	return nil
}

type dCSVRecorder struct {
	w *csv.Writer
}

func (r dCSVRecorder) Record(e Entry) error {
	root, id, parent := e.Root.String(), e.ID.String(), e.Parent.String()
	time := e.Time.Format(time.RFC3339Nano)
	pid := strconv.Itoa(e.PID)

	for k, v := range e.Properties {
		if err := r.w.Write([]string{
			root,
			id,
			parent,
			e.Schema,
			time,
			e.Host,
			pid,
			e.Deploy,
			k,
			v,
		}); err != nil {
			return err
		}

	}

	return nil
}
