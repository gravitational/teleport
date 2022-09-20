package metrics

import (
	"bytes"
)

// Metric is an interface that represents system metric, provides platform-specific escaping
// and is faster than using fmt.Sprintf() as it usually uses bytes.Buffer
type Metric interface {
	// Metric is a composer that creates a new metric by cloning the current metric and adding
	// values to it, is useful for cases with multiple submetric of the same metric
	// m := s.Metric("base")
	// m.Metric("roundtrip")
	// m.Metric("bytes")
	Metric(p ...string) Metric
	String() string
}

func NewMetric(prefix string, p ...string) Metric {
	m := &metric{b: &bytes.Buffer{}}
	if len(prefix) != 0 {
		m.writeString(prefix, true)
	}
	m.writeStrings(p...)
	return m
}

type metric struct {
	b *bytes.Buffer
}

func (m *metric) Metric(p ...string) Metric {
	n := m.makeCopy()
	n.b.WriteRune('.')
	n.writeStrings(p...)
	return n
}

func (m *metric) String() string {
	return m.b.String()
}

func (m *metric) makeCopy() *metric {
	orig := m.b.Bytes()
	new := make([]byte, len(orig))
	copy(new, orig)
	return &metric{b: bytes.NewBuffer(new)}
}

func (m *metric) writeString(s string, addDelimiter bool) {
	writeEscaped(m.b, s, addDelimiter)
}

func (m *metric) writeStrings(p ...string) Metric {
	for i, _ := range p {
		m.writeString(p[i], i != len(p)-1)
	}
	return m
}

func escape(s string) string {
	b := &bytes.Buffer{}
	writeEscaped(b, s, false)
	return b.String()
}

func writeEscaped(b *bytes.Buffer, s string, addDelim bool) {
	b.Grow(len(s) + 1)
	for i := 0; i < len(s); i++ {
		if ('A' <= s[i] && s[i] <= 'Z') || ('a' <= s[i] && s[i] <= 'z') || ('0' <= s[i] && s[i] <= '9') || s[i] == '_' || s[i] == '.' {
			b.WriteByte(s[i])
		} else {
			b.WriteByte('_')
		}
	}
	if addDelim {
		b.WriteRune('.')
	}
}
