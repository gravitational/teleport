// eventually_cache_is_healthy is an Antithesis command which connects to the local metric endpoint and checks the cache
// health metric. There is a recovery deadline [cacheHealthyDeadline] which allows the system recover from faults injected
// prior to this check. All caches should eventually recover.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gravitational/trace"
	dto "github.com/prometheus/client_model/go"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

const (
	metricCacheHealth = "teleport_cache_health"
	// cacheHealthyDeadline is the time allowed for the cache to recover post fault injection being disabled.
	// This does not appear to be explicitly stated anywhere, assume 5min as a starting point.
	cacheHealthyDeadline = 5 * time.Minute
)

func main() {
	level := slog.LevelDebug
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level})))

	if err := run(context.Background()); err != nil {
		fmt.Fprint(os.Stdout, utils.UserMessageFromError(err))
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := stacksignal.GetSignalHandler().NotifyContext(ctx)
	defer cancel()

	var ccf tctlcfg.GlobalCLIFlags
	cfg := servicecfg.MakeDefaultConfig()

	if configFileEnv, ok := os.LookupEnv(defaults.ConfigFileEnvar); ok {
		ccf.ConfigFile = configFileEnv
	} else if utils.FileExists(defaults.ConfigFilePath) {
		ccf.ConfigFile = defaults.ConfigFilePath
	}

	if ccf.ConfigFile == "" {
		return trace.BadParameter("config file is missing")
	}

	fileConf, err := config.ReadConfigFile(ccf.ConfigFile)
	if err != nil {
		return trace.Wrap(err, "reading config file")
	}

	if err = config.ApplyFileConfig(fileConf, cfg); err != nil {
		return trace.Wrap(err, "applying config file")
	}

	clt := debug.NewClient(cfg.DataDir)

	return trace.Wrap(eventually.Assert(ctx, eventually.AssertParams{
		Message:   "cache eventually recovers",
		Timeout:   cacheHealthyDeadline,
		Condition: getCheckCacheHealthFunc(clt),
	}))
}

type MetricsClient interface {
	GetMetrics(ctx context.Context) (map[string]*dto.MetricFamily, error)
}

func getCheckCacheHealthFunc(clt MetricsClient) eventually.ConditionFunc {
	return func(ctx context.Context, addDetail eventually.AddDetailFunc) (bool, error) {
		families, err := clt.GetMetrics(ctx)
		if err != nil {
			return false, trace.Wrap(err, "getting metrics")
		}

		family, ok := families[metricCacheHealth]
		if !ok {
			return false, trace.BadParameter("cache metric %q not found", metricCacheHealth)
		}

		health := map[string]float64{}
		condition := true
		for _, m := range family.GetMetric() {
			component := labelValue(m, teleport.TagCacheComponent)
			value := m.GetGauge().GetValue()
			health[component] = value
			if value < 1.0 {
				condition = false
			}
		}

		addDetail(metricCacheHealth, health)
		return condition, nil
	}
}

func labelValue(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}
