package audit

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPuller(t *testing.T) {
	const (
		auditSID  = "1"
		service   = "XE"
		sessionID = "123"
	)
	entryC := make(chan QueryEntry, 100)
	clock := clockwork.NewFakeClock()
	mockConn := &mockOracleConnector{
		initFunc: func(serviceName, sessionID, addr string, conf *tls.Config) (string, error) {
			return auditSID, nil
		},
	}
	mockConn.fetchAuditLogsFunc = func(audSID string, eid string) ([]QueryEntry, error) {
		switch mockConn.fetchCallCount.Load() {
		case 1:
			return []QueryEntry{
				{Text: "SELECT 1", EntryID: "1"},
			}, nil
		case 2:
			return []QueryEntry{
				{Text: "SELECT 2", EntryID: "2"},
			}, nil
		}
		return nil, nil
	}
	p := &Puller{
		cfg: PullerConfig{
			Interval: time.Second * 5,
			Logger:   slog.New(slog.DiscardHandler),
			oracleDB: mockConn,
			OnQuery: func(entry QueryEntry) {
				entryC <- entry
			},
			clock: clock,
		},
		running: make(chan struct{}),
		close:   make(chan struct{}),
		done:    make(chan struct{}),
	}
	err := p.Init(service, sessionID)
	require.NoError(t, err)
	go func() {
		assert.NoError(t, p.Run(context.Background()))
	}()

	<-p.running
	clock.Advance(time.Second * 5)

	select {
	case <-time.After(time.Second):
		require.Fail(t, "failed to receive entry")
	case e := <-entryC:
		require.Equal(t, "SELECT 1", e.Text)
		require.Equal(t, "1", e.EntryID)
	}

	go func() {
		assert.NoError(t, p.Close())
	}()

	select {
	case <-time.After(time.Second):
		require.Fail(t, "failed to receive entry")
	case e := <-entryC:
		require.Equal(t, "SELECT 2", e.Text)
		require.Equal(t, "2", e.EntryID)
	}
}

func TestPullerClose(t *testing.T) {
	cfg := PullerConfig{
		Addr:      "dummy.addr",
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
		Logger:    slog.New(slog.DiscardHandler),
	}

	p, err := NewPuller(cfg)
	require.NoError(t, err)

	done := make(chan error)

	go func() {
		done <- p.Close()
	}()

	select {
	case errClose := <-done:
		require.NoError(t, errClose)
	case <-time.After(time.Second):
		require.Fail(t, "Close() has blocked")
	}
}

type mockOracleConnector struct {
	fetchCallCount     atomic.Uint32
	initFunc           func(serviceName, sessionID, addr string, conf *tls.Config) (string, error)
	getAudSidFunc      func(sid string) (string, error)
	fetchAuditLogsFunc func(audSID string, eid string) ([]QueryEntry, error)
}

func (m *mockOracleConnector) init(serviceName, sessionID, addr string, conf *tls.Config, kerberosFun KerberosAuthFunc) (string, error) {
	return m.initFunc(serviceName, sessionID, addr, conf)
}

func (m *mockOracleConnector) getAudSid(sid string) (string, error) {
	return m.getAudSidFunc(sid)
}

func (m *mockOracleConnector) fetchAuditLogs(audSID string, eid string) ([]QueryEntry, error) {
	m.fetchCallCount.Add(1)
	return m.fetchAuditLogsFunc(audSID, eid)
}

func (m *mockOracleConnector) close() error {
	return nil
}
