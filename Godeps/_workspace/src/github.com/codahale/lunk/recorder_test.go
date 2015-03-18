package lunk

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"testing"
	"time"
)

func TestNormalizedCSVEntryRecorder(t *testing.T) {
	eB, pB := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	eW, pW := csv.NewWriter(eB), csv.NewWriter(pB)
	eW.Write(NormalizedEventHeaders)
	pW.Write(NormalizedPropertyHeaders)

	r := NewNormalizedCSVEntryRecorder(eW, pW)

	e := Entry{
		EventID: EventID{
			Root:   ID(100),
			ID:     ID(200),
			Parent: ID(150),
		},
		Schema: "event",
		Time:   time.Date(2014, 5, 20, 14, 42, 38, 0, time.UTC),
		Host:   "example.com",
		PID:    600,
		Deploy: "r500",
		Properties: map[string]string{
			"k1": "v1",
			"k2": "v2",
		},
	}

	if err := r.Record(e); err != nil {
		t.Fatal(err)
	}
	eW.Flush()
	pW.Flush()

	eR := csv.NewReader(bytes.NewReader(eB.Bytes()))
	pR := csv.NewReader(bytes.NewReader(pB.Bytes()))

	events, err := eR.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	props, err := pR.ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	expected := [][]string{
		[]string{
			"root",
			"id",
			"parent",
			"schema",
			"time",
			"host",
			"pid",
			"deploy",
		},
		[]string{
			"0000000000000064",
			"00000000000000c8",
			"0000000000000096",
			"event",
			"2014-05-20T14:42:38Z",
			"example.com",
			"600",
			"r500",
		},
	}
	actual := events
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Was %#v but expected %#v", actual, expected)
	}

	expected = [][]string{
		[]string{
			"root",
			"id",
			"parent",
			"prop_name",
			"prop_value",
		},
		[]string{
			"0000000000000064",
			"00000000000000c8",
			"0000000000000096",
			"k1",
			"v1",
		},
		[]string{
			"0000000000000064",
			"00000000000000c8",
			"0000000000000096",
			"k2",
			"v2",
		},
	}
	actual = props
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Was %#v but expected %#v", actual, expected)
	}
}

func TestDenormalizedCSVEntryRecorder(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	w := csv.NewWriter(buf)
	w.Write(DenormalizedEventHeaders)
	r := NewDenormalizedCSVEntryRecorder(w)

	e := Entry{
		EventID: EventID{
			Root:   ID(100),
			ID:     ID(200),
			Parent: ID(150),
		},
		Schema: "event",
		Time:   time.Date(2014, 5, 20, 14, 42, 38, 0, time.UTC),
		Host:   "example.com",
		PID:    600,
		Deploy: "r500",
		Properties: map[string]string{
			"k1": "v1",
			"k2": "v2",
		},
	}

	if err := r.Record(e); err != nil {
		t.Fatal(err)
	}
	w.Flush()

	events, err := csv.NewReader(bytes.NewReader(buf.Bytes())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	expected := [][]string{
		[]string{
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
		},
		[]string{
			"0000000000000064",
			"00000000000000c8",
			"0000000000000096",
			"event",
			"2014-05-20T14:42:38Z",
			"example.com",
			"600",
			"r500",
			"k1",
			"v1",
		},
		[]string{
			"0000000000000064",
			"00000000000000c8",
			"0000000000000096",
			"event",
			"2014-05-20T14:42:38Z",
			"example.com",
			"600",
			"r500",
			"k2",
			"v2",
		},
	}
	actual := events
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Was %#v but expected %#v", actual, expected)
	}
}
