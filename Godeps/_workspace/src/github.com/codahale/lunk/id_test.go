package lunk

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestIDMarshalJSON(t *testing.T) {
	id := ID(10018820)
	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(id)

	actual := `"000000000098e004"`
	expected := strings.TrimSpace(buf.String())

	if actual != expected {
		t.Errorf("Was %v, but was expecting %v", actual, expected)
	}
}

func TestIDUnmarshalJSONHexString(t *testing.T) {
	j := []byte(`"000000000098e004"`)

	var actual ID
	if err := json.Unmarshal(j, &actual); err != nil {
		t.Fatal(err)
	}

	expected := ID(10018820)
	if actual != expected {
		t.Errorf("Was %v, but was expecting %v", actual, expected)
	}
}

func TestIDUnmarshalJSONInt(t *testing.T) {
	j := []byte(`10018820`)

	var actual ID
	if err := json.Unmarshal(j, &actual); err != nil {
		t.Fatal(err)
	}

	expected := ID(10018820)
	if actual != expected {
		t.Errorf("Was %v, but was expecting %v", actual, expected)
	}
}

func TestIDUnmarshalJSONNonInt(t *testing.T) {
	j := []byte(`[]`)

	var actual ID
	err := json.Unmarshal(j, &actual)
	if err == nil {
		t.Fatalf("Unexpectedly unmarshalled %v", actual)
	}
}

func TestIDUnmarshalJSONNonHexString(t *testing.T) {
	j := []byte(`"woo"`)

	var actual ID
	err := json.Unmarshal(j, &actual)
	if err == nil {
		t.Fatalf("Unexpectedly unmarshalled %v", actual)
	}
}

func TestIDGeneration(t *testing.T) {
	n := 10000
	ids := make(map[ID]bool, n)
	for i := 0; i < n; i++ {
		id := generateID()
		if ids[id] {
			t.Errorf("Duplicate ID: %v", id)
		}
		ids[id] = true
	}
}

func TestParseID(t *testing.T) {
	expected := ID(10018181901)
	actual, err := ParseID(expected.String())
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Was %v but expected %v", actual, expected)
	}
}

func TestParseIDError(t *testing.T) {
	id, err := ParseID("woo")
	if err == nil {
		t.Errorf("Unexpectedly parsed value: %v", id)
	}
}

func BenchmarkIDGeneration(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateID()
	}
}
