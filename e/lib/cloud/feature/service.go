package feature

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
)

// Config is the feature service config
type Config struct {
	// Backend is the configured backend
	Backend backend.Backend
	// CloudClient is a client of the cloud API server
	CloudClient cloud.Client
	// Log is the logger
	Log *logrus.Entry
	// Interval is the interval Cloud should be queried for features updates
	Interval time.Duration
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// RequestTimeout is the timeout of the Cloud request
	RequestTimeout time.Duration
}

// CheckAndSetDefaults checks and sets default config values
func (c *Config) CheckAndSetDefaults() error {
	if c.CloudClient == nil {
		return trace.BadParameter("missing Cloud Client")
	}

	if c.Interval <= 0 {
		return trace.BadParameter("Interval value should be greater than 0")
	}

	if c.RequestTimeout <= 0 {
		return trace.BadParameter("RequestTimeout value should be greater than 0")
	}

	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, "cloud.feature")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Service is a service that periodically
// fetch features from Cloud and stores it in the backend
type Service struct {
	backend        backend.Backend
	cloudClient    cloud.Client
	log            *logrus.Entry
	interval       time.Duration
	requestTimeout time.Duration
	clock          clockwork.Clock
}

// NewService returns a new service that periodically fetches
// features from Cloud
func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		backend:        cfg.Backend,
		cloudClient:    cfg.CloudClient,
		log:            cfg.Log,
		interval:       cfg.Interval,
		requestTimeout: cfg.RequestTimeout,
		clock:          cfg.Clock,
	}, nil
}

// Run periodically fetches features from Cloud and reloads the cluster features
// when they change. Blocks the thread.
func (s *Service) Run(ctx context.Context) error {
	s.log.WithField("interval", s.interval).Info("Feature service has started")
	ticker := s.clock.NewTicker(s.interval)

	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			// fetch
			s.log.Info("Fetching Cloud features")
			f, err := s.getFeatures(ctx)
			if err != nil {
				s.log.Errorf("Failed fetching cloud features: %+v", err)
				continue
			}

			// update cluster features
			modules.GetModules().SetFeatures(*f)

			// store in the backend
			_, err = Store(ctx, *f, s.backend)
			if err != nil {
				s.log.Errorf("Failed storing features in the backend: %+v", err)
				continue
			}
			s.log.Infof("Done updating cluster features: %+v", f)
		case <-ctx.Done():
			s.log.Info("Feature service has stopped")
			return nil
		}
	}
}

func (s *Service) getFeatures(ctx context.Context) (*modules.Features, error) {
	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()
	return GetCloudFeatures(ctx, s.cloudClient)
}
