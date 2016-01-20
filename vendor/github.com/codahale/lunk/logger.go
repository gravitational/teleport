package lunk

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// An EventLogger logs events and their metadata.
type EventLogger interface {
	// Log adds the given event to the log stream.
	Log(id EventID, e Event)
}

// NewJSONEventLogger returns an EventLogger which writes entries as streaming
// JSON to the given writer.
func NewJSONEventLogger(w io.Writer) EventLogger {
	return jsonEventLogger{json.NewEncoder(w)}
}

// NewTextEventLogger returns an EventLogger which writes entries as single
// lines of attr="value" formatted text.
func NewTextEventLogger(w io.Writer) EventLogger {
	return textEventLogger{w: w}
}

// A SamplingEventLogger logs a uniform sampling of events, if configured to do
// so.
type SamplingEventLogger struct {
	l         EventLogger
	r         *rand.Rand
	rates     map[string]float64
	rootRates map[ID]float64
	m         *sync.Mutex
}

// NewSamplingEventLogger returns a new SamplingEventLogger, passing events
// through to the given EventLogger.
func NewSamplingEventLogger(l EventLogger) *SamplingEventLogger {
	return &SamplingEventLogger{
		l:         l,
		r:         rand.New(rand.NewSource(time.Now().UnixNano())),
		rates:     make(map[string]float64),
		rootRates: make(map[ID]float64),
		m:         new(sync.Mutex),
	}
}

// SetRootSampleRate sets the sampling rate for all events with the given root
// ID. p should be between 0.0 (no events logged) and 1.0 (all events logged),
// inclusive.
func (l SamplingEventLogger) SetRootSampleRate(root ID, p float64) {
	l.m.Lock()
	defer l.m.Unlock()

	l.rootRates[root] = p
}

// UnsetRootSampleRate removes any settings for events with the given root ID.
func (l SamplingEventLogger) UnsetRootSampleRate(root ID) {
	l.m.Lock()
	defer l.m.Unlock()

	delete(l.rootRates, root)
}

// SetSchemaSampleRate sets the sampling rate for all events with the given
// schema. p should be between 0.0 (no events logged) and 1.0 (all events
// logged), inclusive.
func (l SamplingEventLogger) SetSchemaSampleRate(schema string, p float64) {
	l.m.Lock()
	defer l.m.Unlock()

	l.rates[schema] = p
}

// UnsetSchemaSampleRate removes any settings for events with the given root ID.
func (l SamplingEventLogger) UnsetSchemaSampleRate(schema string) {
	l.m.Lock()
	defer l.m.Unlock()

	delete(l.rates, schema)
}

// Log passes the event to the underlying EventLogger, probabilistically
// dropping some events.
func (l SamplingEventLogger) Log(id EventID, e Event) {
	l.m.Lock()
	defer l.m.Unlock()

	r, ok := l.rootRates[id.Root]
	if !ok {
		r, ok = l.rates[e.Schema()]
	}

	if ok && r < l.r.Float64() {
		return
	}

	l.l.Log(id, e)
}

type jsonEventLogger struct {
	*json.Encoder
}

func (l jsonEventLogger) Log(id EventID, e Event) {
	if err := l.Encode(NewEntry(id, e)); err != nil {
		panic(err)
	}
}

type textEventLogger struct {
	w io.Writer
}

func (l textEventLogger) Log(id EventID, e Event) {
	entry := NewEntry(id, e)

	props := []string{
		fmt.Sprintf("time=%s", strconv.Quote(entry.Time.Format(time.RFC3339))),
		fmt.Sprintf("host=%s", strconv.Quote(entry.Host)),
		fmt.Sprintf(`pid="%d"`, entry.PID),
		fmt.Sprintf("deploy=%s", strconv.Quote(entry.Deploy)),
		fmt.Sprintf("schema=%s", strconv.Quote(entry.Schema)),
		fmt.Sprintf("id=%s", strconv.Quote(entry.ID.String())),
		fmt.Sprintf("root=%s", strconv.Quote(entry.Root.String())),
	}

	if entry.Parent != 0 {
		s := fmt.Sprintf("parent=%s", strconv.Quote(entry.Parent.String()))
		props = append(props, s)
	}

	for _, k := range sortedKeys(entry.Properties) {
		s := fmt.Sprintf("p:%s=%s", k, strconv.Quote(entry.Properties[k]))
		props = append(props, s)
	}

	if _, err := fmt.Fprintln(l.w, strings.Join(props, " ")); err != nil {
		panic(err)
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
