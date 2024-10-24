package audit

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
)

// NewPuller create a new instance of Puller.
func NewPuller(cfg PullerConfig) (*Puller, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Puller{
		cfg:     cfg,
		close:   make(chan struct{}),
		done:    make(chan struct{}),
		running: make(chan struct{}),
	}, nil
}

// CheckAndSetDefaults checks and sets defaults.
func (a *PullerConfig) CheckAndSetDefaults() error {
	if a.Addr == "" {
		return trace.BadParameter("missing addr")
	}
	if a.TLSConfig == nil {
		return trace.BadParameter("missing tlsConf")
	}
	if a.Logger == nil {
		a.Logger = slog.With(teleport.ComponentKey, "DB:ORC:AU")
	}
	if a.Interval <= 0 {
		a.Interval = time.Second * 20
	}
	if a.oracleDB == nil {
		a.oracleDB = &oracleDB{}
	}

	if a.clock == nil {
		a.clock = clockwork.NewRealClock()
	}
	return nil
}

// Puller allows to monitor Oracle audit table and fetches entries periodically.
type Puller struct {
	cfg PullerConfig
	// audit session ID identifier.
	audSID      string
	lastEntryID string
	close       chan struct{}
	done        chan struct{}
	running     chan struct{}
	mtx         sync.Mutex
}

// PullerConfig is a configuration of Puller.
type PullerConfig struct {
	// Addr is the Oracle instance address.
	Addr string
	// TLSConfig is the TLSConfig uses to auth via TCPS listener.
	TLSConfig *tls.Config
	// OnQuery is the callback function called on each audit entry record.
	OnQuery func(QueryEntry)
	// Logger is used for logging.
	Logger *slog.Logger
	// Interval is a pull interval for audit fetcher.
	Interval time.Duration

	oracleDB oracleConnector
	clock    clockwork.Clock
}

// Init initializes the Puller state.
func (a *Puller) Init(serviceName string, sessionID string) error {
	audSID, err := a.cfg.oracleDB.init(serviceName, sessionID, a.cfg.Addr, a.cfg.TLSConfig)
	if err != nil {
		close(a.done)
		return trace.Wrap(err)
	}
	a.audSID = audSID
	a.cfg.Logger.DebugContext(context.Background(), "Initialize Active Audit Fetcher", "audit_session_id", audSID, "session_id", sessionID)
	return nil
}

// Run runs the Puller.
func (a *Puller) Run(ctx context.Context) error {
	defer func() { close(a.done) }()
	defer func() { a.cfg.oracleDB.close() }()

	if err := a.runLoop(ctx); err != nil {
		return trace.Wrap(err)
	}

	// runLoop exits where client or server connection is closed.
	// fetch audit logs one more time to ensure that all the client entries
	// were fetched.
	if err := a.fetchAndProcessAuditLogs(ctx); err != nil {
		return trace.Wrap(err)
	}
	a.cfg.Logger.DebugContext(ctx, "Oracle Audit log Puller closing", "audit_session_id", a.audSID)
	return nil
}

// Close closes the Puller via blocking call and ensure that the last audit fetch call was triggered.
func (a *Puller) Close() error {
	// indicate that fetch call should be triggered immediately.
	close(a.close)
	// Block and wait for the last audit logs fetch call.
	<-a.done
	return nil
}

func (a *Puller) runLoop(ctx context.Context) error {
	tc := a.cfg.clock.NewTicker(a.cfg.Interval)
	defer func() { tc.Stop() }()
	close(a.running)
	for {
		select {
		case <-a.close:
			return nil
		case <-ctx.Done():
			return nil
		case <-tc.Chan():
			if err := a.fetchAndProcessAuditLogs(ctx); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func (a *Puller) fetchAndProcessAuditLogs(ctx context.Context) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	queryEntries, err := a.cfg.oracleDB.fetchAuditLogs(a.audSID, a.lastEntryID)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(queryEntries) > 0 {
		a.cfg.Logger.DebugContext(ctx, "Processing audit entries", "audit_session_id", a.audSID, "entry_count", len(queryEntries), "last_entry_id", a.lastEntryID)
	}
	for _, q := range queryEntries {
		a.lastEntryID = q.EntryID
		a.cfg.OnQuery(q)
	}
	return nil
}
