package lunk

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestNewRootEventID(t *testing.T) {
	id := NewRootEventID()

	if id.Parent != 0 {
		t.Errorf("Unexpected parent: %+v", id)
	}

	if id.ID == 0 {
		t.Errorf("Zero ID: %+v", id)
	}

	if id.Root == 0 {
		t.Errorf("Zero root: %+v", id)
	}

	if id.Root == id.ID {
		t.Errorf("Duplicate IDs: %+v", id)
	}
}

func TestNewEventID(t *testing.T) {
	root := NewRootEventID()
	id := NewEventID(root)

	if id.Parent != root.ID {
		t.Errorf("Unexpected parent: %+v", id)
	}

	if id.ID == 0 {
		t.Errorf("Zero ID: %+v", id)
	}

	if id.Root != root.Root {
		t.Errorf("Mismatched root: %+v", id)
	}
}

func TestEventIDString(t *testing.T) {
	id := EventID{
		Root: 100,
		ID:   300,
	}

	actual := id.String()
	expected := "0000000000000064/000000000000012c"
	if actual != expected {
		t.Errorf("Was %#v, but expected %#v", actual, expected)
	}
}

func TestEventIDStringWithParent(t *testing.T) {
	id := EventID{
		Root:   100,
		Parent: 200,
		ID:     300,
	}

	actual := id.String()
	expected := "0000000000000064/000000000000012c/00000000000000c8"
	if actual != expected {
		t.Errorf("Was %#v, but expected %#v", actual, expected)
	}
}

func TestEventIDFormat(t *testing.T) {
	id := EventID{
		Root: 100,
		ID:   300,
	}

	actual := id.Format("/* %s */ %s", "SELECT 1")
	expected := "/* 0000000000000064/000000000000012c */ SELECT 1"
	if actual != expected {
		t.Errorf("Was %#v, but expected %#v", actual, expected)
	}
}

func ExampleEventID_Format() {
	// Assume we're connected to a database.
	var (
		event  EventID
		db     *sql.DB
		userID int
	)

	// This passes the root ID and the parent event ID to the database, which
	// allows us to correlate, for example, slow queries with the web requests
	// which caused them.
	query := event.Format(`/* %s/%s */ %s`, `SELECT email FROM users WHERE id = ?`)
	r := db.QueryRow(query, userID)
	if r == nil {
		panic("user not found")
	}

	var email string
	if err := r.Scan(&email); err != nil {
		panic("couldn't read email")
	}

	fmt.Printf("User's email: %s\n", email)
}

func TestParseEventID(t *testing.T) {
	id, err := ParseEventID("0000000000000064/000000000000012c")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if id.Root != 100 || id.ID != 300 {
		t.Errorf("Unexpected event ID: %+v", id)
	}
}

func TestParseEventIDWithParent(t *testing.T) {
	id, err := ParseEventID("0000000000000064/000000000000012c/0000000000000096")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if id.Root != 100 || id.Parent != 150 || id.ID != 300 {
		t.Errorf("Unexpected event ID: %+v", id)
	}
}

func TestParseEventIDMalformed(t *testing.T) {
	id, err := ParseEventID(`0000000000000064000000000000012c`)

	if id != nil {
		t.Errorf("Unexpected event ID: %+v", id)
	}

	if err != ErrBadEventID {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseEventIDBadRoot(t *testing.T) {
	id, err := ParseEventID("0000000000g000064/000000000000012c")

	if id != nil {
		t.Errorf("Unexpected event ID: %+v", id)
	}

	if err != ErrBadEventID {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseEventIDBadID(t *testing.T) {
	id, err := ParseEventID("0000000000000064/0000000000g00012c")

	if id != nil {
		t.Errorf("Unexpected event ID: %+v", id)
	}

	if err != ErrBadEventID {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseEventIDBadParent(t *testing.T) {
	id, err := ParseEventID("0000000000000064/000000000000012c/00000000000g0096")

	if id != nil {
		t.Errorf("Unexpected event ID: %+v", id)
	}

	if err != ErrBadEventID {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNewEntry(t *testing.T) {
	id := NewRootEventID()
	e := NewEntry(id, mockEvent{Example: "yay"})

	if e.EventID != id {
		t.Errorf("Was %v but expected %v", e.EventID, id)
	}

	if e.Schema != "example" {
		t.Errorf("Unexpected schema: %v", e.Schema)
	}

	if time.Now().Sub(e.Time) > 5*time.Millisecond {
		t.Errorf("Unexpectedly old timestamp: %v", e.Time)
	}

	if e.Time.Location() != time.UTC {
		t.Errorf("Unexpectedly non-UTC timestamp: %v", e.Time)
	}

	if e.Host == "" {
		t.Errorf("Blank hostname for meta data")
	}

	if e.PID == 0 {
		t.Errorf("Blank PID for meta data")
	}

	expected := map[string]string{
		"example": "yay",
	}
	if !reflect.DeepEqual(e.Properties, expected) {
		t.Errorf("Was %+v, but expected %+v", e.Properties, expected)
	}
}

type mockEvent struct {
	Example string
}

func (mockEvent) Schema() string {
	return "example"
}
