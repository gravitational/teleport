package memlog

import (
	"encoding/json"
)

type Logger interface {
	Write([]byte) (int, error)
	LastEvents() []interface{}
}

func New() Logger {
	return &logger{
		entries: []interface{}{},
	}
}

type logger struct {
	entries []interface{}
}

func (l *logger) Write(v []byte) (int, error) {
	var i interface{}
	if err := json.Unmarshal(v, &i); err != nil {
		return 0, err
	}
	l.entries = append([]interface{}{i}, l.entries...)
	return len(v), nil
}

func (l *logger) LastEvents() []interface{} {
	if len(l.entries) < 100 {
		return l.entries[:len(l.entries)]
	}
	return l.entries[:100]
}
