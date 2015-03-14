package web

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
)

var _ lunk.Event = HTTPRequestEvent{}

func TestSetRequestEventID(t *testing.T) {
	r := http.Request{
		Header: http.Header{},
	}

	SetRequestEventID(&r, lunk.EventID{
		Root: 100,
		ID:   150,
	})

	actual := r.Header.Get("Event-ID")
	expected := "0000000000000064/0000000000000096"
	if actual != expected {
		t.Errorf("Was %#v, but expected %#v", actual, expected)
	}
}

func TestGetRequestEventID(t *testing.T) {
	r := http.Request{
		Header: http.Header{},
	}
	r.Header.Add("Event-ID", "0000000000000064/0000000000000096")

	id, err := GetRequestEventID(&r)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if id.Root != 100 || id.ID != 150 {
		t.Errorf("Unexpected event ID: %+v", id)
	}
}

func TestGetRequestEventIDMissing(t *testing.T) {
	r := http.Request{
		Header: http.Header{},
	}

	id, err := GetRequestEventID(&r)

	if id != nil {
		t.Errorf("Unexpected event ID: %+v", id)
	}

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestHTTPRequest(t *testing.T) {
	r := &http.Request{
		Host:          "example.com",
		Method:        "GET",
		RequestURI:    "/woohoo",
		Proto:         "HTTP/1.1",
		RemoteAddr:    "127.0.0.1",
		ContentLength: 0,
		Header: http.Header{
			"Authorization": []string{"Basic seeecret"},
			"Accept":        []string{"application/json"},
		},
		Trailer: http.Header{
			"Authorization": []string{"Basic seeecret"},
			"Connection":    []string{"close"},
		},
	}

	e := HTTPRequest(r)
	e.Status = 200
	e.Elapsed = 4300 * time.Microsecond

	if e.Schema() != "httprequest" {
		t.Errorf("Unexpected schema: %v", e.Schema())
	}

	actual := lunk.NewEntry(lunk.NewRootEventID(), e).Properties
	expected := map[string]string{
		"elapsed":               "4.3",
		"headers.connection":    "close",
		"headers.accept":        "application/json",
		"headers.authorization": "REDACTED",
		"proto":                 "HTTP/1.1",
		"remote_addr":           "127.0.0.1",
		"host":                  "example.com",
		"content_length":        "0",
		"status":                "200",
		"method":                "GET",
		"uri":                   "/woohoo",
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Was %#v, but expected %#v", actual, expected)
	}
}
