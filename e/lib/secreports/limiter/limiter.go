package limiter

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/utils/retryutils"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/secreports/metrics"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// defaultPreAllocationValue is the default value for pre-allocated scanned bytes per single query.
	defaultPreAllocationValue = 34359738368 // 32GB
	// defaultRefillAfter is the default value for the time after which the limit is reset.
	defaultRefillAfter = time.Hour * 24 * 30 // 30 days
	// DefaultName is the default limiter name.
	defaultName = "athena_cost_limiter"
)

// Config is the limiter configuration.
type Config struct {
	// Store is the security reports store.
	Store services.CostLimiter
	// Semaphore is the semaphore service.
	Semaphore types.Semaphores
	// Log is the logger.
	Log logrus.FieldLogger
	// Clock is the clock.
	Clock clockwork.Clock
	// Name is the limiter name.
	Name string
	// RefillAfter is the time after which the limit is reset.
	RefillAfter time.Duration
	// PreAllocationValue is the value for pre-allocated scanned bytes per single query.
	PreAllocationValue uint64
	// TotalLimit is the total limit of scanned bytes.
	TotalLimit uint64
}

const (
	// TB is 1 TB unit.
	TB = uint64(1099511627776) // 1TB
)

// CheckAndSetDefaults checks and sets default parameters.
func (l *Config) CheckAndSetDefaults() error {
	if l.Clock == nil {
		l.Clock = clockwork.NewRealClock()
	}
	if l.Semaphore == nil {
		return trace.BadParameter("missing Semaphore")
	}
	if l.Store == nil {
		return trace.BadParameter("missing Store")
	}
	if l.Log == nil {
		l.Log = logrus.New()
	}
	if l.RefillAfter == 0 {
		l.RefillAfter = defaultRefillAfter
	}

	if l.PreAllocationValue == 0 {
		l.PreAllocationValue = defaultPreAllocationValue
	}
	if l.TotalLimit == 0 {
		l.TotalLimit = getDefaultLimit()
	}
	if l.Name == "" {
		l.Name = defaultName
	}
	return nil
}

// NewLimiter creates a new limiter.
func NewLimiter(cfg Config) (*Limiter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Limiter{
		Config: cfg,
	}, nil
}

// Limiter is the limiter. Limit number of scanned bytes per RefillAfter period
// for Athena queries to avoid overcharging.
type Limiter struct {
	Config
	mtx sync.Mutex
}

func (l *Limiter) withLock(ctx context.Context, call func() error) error {
	// In cse of running the process in the same process lock mutex to avoid semaphore acquisition retry calls.
	l.mtx.Lock()
	defer l.mtx.Unlock()

	lease, err := services.AcquireSemaphoreWithRetry(ctx, services.AcquireSemaphoreWithRetryConfig{
		Service: l.Semaphore,
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.SemaphoreKindAccessMonitoringLimiter,
			SemaphoreName: l.Name,
			MaxLeases:     1,
			Expires:       l.Clock.Now().Add(time.Minute),
		},
		Retry: retryutils.LinearConfig{
			Step:  time.Second,
			Max:   time.Second,
			Clock: l.Clock,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := l.Semaphore.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			l.Log.WithError(err).Errorf("Failed to cancel lease: %v.", lease)
		}
	}()
	if err := call(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (l *Limiter) allocateLimit(ctx context.Context) error {
	err := l.withLock(ctx, func() error {
		item, err := l.Store.GetCostLimiter(ctx, l.Name)
		if err != nil {
			if trace.IsNotFound(err) {
				item, err = secreports.NewCostLimiter(header.Metadata{Name: l.Name}, secreports.CostLimiterSpec{
					RefillAt:    l.Clock.Now().Add(l.RefillAfter),
					RefillAfter: l.RefillAfter,
				})
				if err != nil {
					return trace.Wrap(err)
				}
				if err := l.Store.UpsertCostLimiter(ctx, item); err != nil {
					return trace.Wrap(err)
				}
			} else {
				return trace.Wrap(err)
			}
		}
		l.maybeUpdateLimit(item)

		if item.Spec.BytesScanned >= l.TotalLimit {
			return trace.LimitExceeded("scanned byte limit %v exceeded", l.TotalLimit)
		}
		item.Spec.BytesScanned += l.PreAllocationValue
		if err := l.Store.UpsertCostLimiter(ctx, item); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return trace.Wrap(err)
}

func (l *Limiter) updateQueryLimit(ctx context.Context, scannedBytes uint64) error {
	err := l.withLock(ctx, func() error {
		item, err := l.Store.GetCostLimiter(ctx, l.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		item.Spec.BytesScanned += scannedBytes
		if item.Spec.BytesScanned >= l.PreAllocationValue {
			item.Spec.BytesScanned -= l.PreAllocationValue
		}
		if err := l.Store.UpsertCostLimiter(ctx, item); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AllocateLimit checks is limit is exceeded and allocates
// the PreAllocationValue limit for a query before query execution.
// Function returns a callback function that should be called after query execution to applied real
// query scanned bytes value to the current usage.
func (l *Limiter) AllocateLimit(ctx context.Context) (func(scanned uint64) error, error) {
	if err := l.allocateLimit(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return func(scanned uint64) error {
		return trace.Wrap(l.updateQueryLimit(ctx, scanned))
	}, nil
}

// Details is the limiter details.
type Details struct {
	// Current is the current number of scanned bytes.
	Current uint64
	// Limit is the total limit of scanned bytes.
	Limit uint64
	// Start is the start time of the current period.
	Start time.Time
	// End is the end time of the current period.
	End time.Time
}

func (l *Limiter) maybeUpdateLimit(item *secreports.CostLimiter) bool {
	if !item.Spec.RefillAt.Before(l.Clock.Now()) {
		return false
	}
	l.Log.Debug("Resetting limiter.")
	item.Reset(l.Clock.Now().Add(l.RefillAfter))
	item.Spec.RefillAfter = l.RefillAfter
	return true
}

// GetDetails returns the limiter details.
// Additionally, it updates the limiter if needed.
func (l *Limiter) GetDetails(ctx context.Context) (*Details, error) {
	var item *secreports.CostLimiter
	err := l.withLock(ctx, func() error {
		var err error
		item, err = l.Store.GetCostLimiter(ctx, l.Name)
		if err != nil {
			return trace.Wrap(err)
		}
		if l.maybeUpdateLimit(item) {
			if err := l.Store.UpsertCostLimiter(ctx, item); err != nil {
				return trace.Wrap(err)
			}
		}
		if l.TotalLimit != 0 {
			metrics.LimitUsage.Set(float64(item.Spec.BytesScanned) / float64(l.TotalLimit))
		}
		metrics.LimitRefillTimestamp.Set(float64(item.Spec.RefillAt.Unix()))
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Details{
		Current: item.Spec.BytesScanned,
		Limit:   l.TotalLimit,
		Start:   item.Spec.RefillAt.Add(-l.RefillAfter),
		End:     item.Spec.RefillAt,
	}, nil
}

// UpdateLimit updates the total limit of scanned bytes.
func (l *Limiter) updateLimit(limit uint64) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.TotalLimit = limit
}

type cloudClientGetter interface {
	GetCloudClient() cloudapi.TenantsServiceClient
}

// UpdateLimiterBasedOnCloudProduct updates the limiter based on the Cloud product.
// The Features obtained from License doesn't provide information about the Cloud product.
// For that reason BillingInformation are fetched.
func (l *Limiter) UpdateLimiterBasedOnCloudProduct(ctx context.Context, clientGetter cloudClientGetter) {
	if !modules.GetModules().Features().Cloud {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
			client := clientGetter.GetCloudClient()
			if client == nil {
				continue
			}
			resp, err := client.GetBillingInformation(ctx, &cloudapi.EmptyRequest{})
			if err != nil {
				l.Log.Debug("Failed to get billing information: %v.", err)
				continue
			}
			l.updateLimit(cloudLimits(resp))
			l.Log.Info("Access Monitoring Limiter.TotalLimits updated")
			return
		}
	}
}

const (
	// For trial Cloud product Monthly Data Limit is 1TB
	trialProductLimit = TB / 5 // 1 $
	// For Team Cloud product Monthly Data Limit is 3TB
	teamProductLimit = 3 * TB // 15 $
	// For Enterprise Cloud product Monthly Data Limit is 20TB
	enterpriseProductLimit = 20 * TB // 100 $
)

// getDefaultMonthlyDataLimit returns monthly data limit based on the product type.
// Teleport Enterprise Cloud:
//   - Trial: 1TB
//   - Team: 3TB
//   - Enterprise: 20TB
//
// TODO(smallinksy): Ideally this limits should be managed by Sales Center.
// todo (michellescripts) add this to sales center
func cloudLimits(resp *cloudapi.GetBillingInformationResponse) uint64 {
	f := modules.GetModules().Features()
	switch {
	case resp.Trial:
		return trialProductLimit
	case f.ProductType == modules.ProductTypeTeam:
		return teamProductLimit
	default:
		return enterpriseProductLimit
	}
}

func getDefaultLimit() uint64 {
	if modules.GetModules().Features().Cloud {
		return trialProductLimit
	}
	return enterpriseProductLimit
}
