package lunk

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
)

// An Event is a record of the occurrence of something.
type Event interface {
	// Schema returns the schema of the event. This should be constant.
	Schema() string
}

var (
	// ErrBadEventID is returned when the event ID cannot be parsed.
	ErrBadEventID = errors.New("bad event ID")
)

// EventID is the ID of an event, its parent event, and its root event.
type EventID struct {
	// Root is the root ID of the tree which contains all of the events related
	// to this one.
	Root ID `json:"root"`

	// ID is an ID uniquely identifying the event.
	ID ID `json:"id"`

	// Parent is the ID of the parent event, if any.
	Parent ID `json:"parent,omitempty"`
}

// String returns the EventID as a slash-separated, set of hex-encoded
// parameters (root, ID, parent). If the EventID has no parent, that value is
// elided.
func (id EventID) String() string {
	if id.Parent == 0 {
		return fmt.Sprintf("%s%s%s", id.Root, EventIDDelimiter, id.ID)
	}
	return fmt.Sprintf(
		"%s%s%s%s%s",
		id.Root,
		EventIDDelimiter,
		id.ID,
		EventIDDelimiter,
		id.Parent,
	)
}

// Format formats according to a format specifier and returns the resulting
// string. The receiver's string representation is the first argument.
func (id EventID) Format(s string, args ...interface{}) string {
	args = append([]interface{}{id.String()}, args...)
	return fmt.Sprintf(s, args...)
}

// NewRootEventID generates a new event ID for a root event. This should only be
// used to generate entries for events caused exclusively by events which are
// outside of your system as a whole (e.g., a root event for the first time you
// see a user request).
func NewRootEventID() EventID {
	return EventID{
		Root: generateID(),
		ID:   generateID(),
	}
}

// NewEventID returns a new ID for an event which is the child of the given
// parent ID. This should be used to track causal relationships between events.
func NewEventID(parent EventID) EventID {
	return EventID{
		Root:   parent.Root,
		ID:     generateID(),
		Parent: parent.ID,
	}
}

const (
	// EventIDDelimiter is the delimiter used to concatenate an EventID's
	// components.
	EventIDDelimiter = "/"
)

// ParseEventID parses the given string as a slash-separated set of parameters.
func ParseEventID(s string) (*EventID, error) {
	parts := strings.Split(s, EventIDDelimiter)
	if len(parts) != 2 && len(parts) != 3 {
		return nil, ErrBadEventID
	}

	root, err := ParseID(parts[0])
	if err != nil {
		return nil, ErrBadEventID
	}

	id, err := ParseID(parts[1])
	if err != nil {
		return nil, ErrBadEventID
	}

	var parent ID
	if len(parts) == 3 {
		i, err := ParseID(parts[2])
		if err != nil {
			return nil, ErrBadEventID
		}
		parent = i
	}

	return &EventID{
		Root:   root,
		ID:     id,
		Parent: parent,
	}, nil
}

// An Entry is the combination of an event and its metadata.
type Entry struct {
	EventID

	// Schema is the schema of the event.
	Schema string `json:"schema"`

	// Time is the timestamp of the event.
	Time time.Time `json:"time"`

	// Host is the name of the host on which the event occurred.
	Host string `json:"host,omitempty"`

	// Deploy is the ID of the deployed artifact, read from the DEPLOY
	// environment variable on startup.
	Deploy string `json:"deploy,omitempty"`

	// PID is the process ID which generated the event.
	PID int `json:"pid"`

	// Properties are the flattened event properties.
	Properties map[string]string `json:"properties"`
}

// NewEntry creates a new entry for the given ID and event.
func NewEntry(id EventID, e Event) Entry {
	props := make(map[string]string, 10)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		props[k] = v
	})

	return Entry{
		EventID:    id,
		Schema:     e.Schema(),
		Time:       time.Now().In(time.UTC),
		Host:       host,
		Deploy:     deploy,
		PID:        pid,
		Properties: props,
	}
}

var (
	host, deploy string
	pid          int
)

func init() {
	h, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	host = h
	deploy = os.Getenv("DEPLOY")
	pid = os.Getpid()
}
