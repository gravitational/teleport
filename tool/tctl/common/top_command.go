/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/dustin/go-humanize"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

// TopCommand implements `tctl top` group of commands.
type TopCommand struct {
	config *servicecfg.Config

	// CLI clauses (subcommands)
	top           *kingpin.CmdClause
	diagURL       *string
	refreshPeriod *time.Duration
}

// Initialize allows TopCommand to plug itself into the CLI parser.
func (c *TopCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config
	c.top = app.Command("top", "Report diagnostic information.")
	c.diagURL = c.top.Arg("diag-addr", "Diagnostic HTTP URL").Default("http://127.0.0.1:3000").String()
	c.refreshPeriod = c.top.Arg("refresh", "Refresh period").Default("5s").Duration()
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *TopCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case c.top.FullCommand():
		diagClient, err := roundtrip.NewClient(*c.diagURL, "")
		if err != nil {
			return true, trace.Wrap(err)
		}
		err = c.Top(ctx, diagClient)
		if trace.IsConnectionProblem(err) {
			return true, trace.ConnectionProblem(err,
				"[CLIENT] Could not connect to metrics service at %v. Is teleport running with --diag-addr=%v?", *c.diagURL, *c.diagURL)
		}
		return true, trace.Wrap(err)
	default:
		return false, nil
	}
}

// Top is called to execute "status" CLI command.
func (c *TopCommand) Top(ctx context.Context, client *roundtrip.Client) error {
	if err := ui.Init(); err != nil {
		return trace.Wrap(err)
	}
	defer ui.Close()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(*c.refreshPeriod)
	defer ticker.Stop()

	// fetch and render first time
	var prev *Report
	re, err := c.fetchAndGenerateReport(ctx, client, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	lastTab := ""
	if err := c.render(ctx, *re, lastTab); err != nil {
		return trace.Wrap(err)
	}
	for {
		select {
		case e := <-uiEvents:
			switch e.ID { // event string/identifier
			case "q", "<C-c>": // press 'q' or 'C-c' to quit
				return nil
			}
			if e.ID == "1" || e.ID == "2" || e.ID == "3" || e.ID == "4" {
				lastTab = e.ID
			}
			// render previously fetched data on the resize event
			if re != nil {
				if err := c.render(ctx, *re, lastTab); err != nil {
					return trace.Wrap(err)
				}
			}
		case <-ticker.C:
			// fetch data and re-render on ticker
			prev = re
			re, err = c.fetchAndGenerateReport(ctx, client, prev)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := c.render(ctx, *re, lastTab); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func (c *TopCommand) render(ctx context.Context, re Report, eventID string) error {
	h := widgets.NewParagraph()
	h.Text = fmt.Sprintf("Report Generated at %v for host %v. Press <q> or Ctrl-C to quit.",
		re.Timestamp.Format(constants.HumanDateFormatSeconds), re.Hostname)
	h.Border = false
	h.TextStyle = ui.NewStyle(ui.ColorMagenta)

	termWidth, termHeight := ui.TerminalDimensions()

	backendRequestsTable := func(title string, b BackendStats) *widgets.Table {
		t := widgets.NewTable()
		t.Title = title
		t.TitleStyle = ui.NewStyle(ui.ColorCyan)
		t.ColumnWidths = []int{10, 10, 10, 50000}
		t.RowSeparator = false
		t.Rows = [][]string{
			{"Count", "Req/Sec", "Range", "Key"},
		}
		for _, req := range b.SortedTopRequests() {
			t.Rows = append(t.Rows,
				[]string{
					humanize.FormatFloat("", float64(req.Count)),
					humanize.FormatFloat("", req.GetFreq()),
					fmt.Sprintf("%v", req.Key.IsRange()),
					req.Key.Key,
				})
		}
		return t
	}

	eventsTable := func(w *WatcherStats) *widgets.Table {
		t := widgets.NewTable()
		t.Title = "Top Events Emitted"
		t.TitleStyle = ui.NewStyle(ui.ColorCyan)
		t.ColumnWidths = []int{10, 10, 10, 50000}
		t.RowSeparator = false
		t.Rows = [][]string{
			{"Count", "Req/Sec", "Avg Size", "Resource"},
		}
		for _, event := range w.SortedTopEvents() {
			t.Rows = append(t.Rows,
				[]string{
					humanize.FormatFloat("", float64(event.Count)),
					humanize.FormatFloat("", event.GetFreq()),
					humanize.FormatFloat("", event.AverageSize()),
					event.Resource,
				})
		}
		return t
	}

	eventsGraph := func(title string, buf *utils.CircularBuffer) *widgets.Plot {
		lc := widgets.NewPlot()
		lc.Title = title
		lc.TitleStyle = ui.NewStyle(ui.ColorCyan)
		lc.Data = make([][]float64, 1)
		// only get the most recent events to fill the graph
		lc.Data[0] = buf.Data((termWidth / 2) - 10)
		lc.AxesColor = ui.ColorWhite
		lc.LineColors[0] = ui.ColorGreen
		lc.Marker = widgets.MarkerDot

		return lc
	}

	t1 := widgets.NewTable()
	t1.Title = "Cluster Stats"
	t1.TitleStyle = ui.NewStyle(ui.ColorCyan)
	t1.ColumnWidths = []int{30, 50000}
	t1.RowSeparator = false
	t1.Rows = [][]string{
		{"Interactive Sessions", humanize.FormatFloat("", re.Cluster.InteractiveSessions)},
		{"Cert Gen Active Requests", humanize.FormatFloat("", re.Cluster.GenerateRequests)},
		{"Cert Gen Requests/sec", humanize.FormatFloat("", re.Cluster.GenerateRequestsCount.GetFreq())},
		{"Cert Gen Throttled Requests/sec", humanize.FormatFloat("", re.Cluster.GenerateRequestsThrottledCount.GetFreq())},
		{"Auth Watcher Queue Size", humanize.FormatFloat("", re.Cache.QueueSize)},
		{"Active Migrations", strings.Join(re.Cluster.ActiveMigrations, ", ")},
	}
	for _, rc := range re.Cluster.RemoteClusters {
		t1.Rows = append(t1.Rows, []string{
			fmt.Sprintf("Cluster %v", rc.Name), rc.IsConnected(),
		})
	}

	t2 := widgets.NewTable()
	t2.Title = "Process Stats"
	t2.TitleStyle = ui.NewStyle(ui.ColorCyan)
	t2.ColumnWidths = []int{30, 50000}
	t2.RowSeparator = false
	t2.Rows = [][]string{
		{"Start Time", re.Process.StartTime.Format(constants.HumanDateFormatSeconds)},
		{"Resident Memory Bytes", humanize.Bytes(uint64(re.Process.ResidentMemoryBytes))},
		{"Open File Descriptors", humanize.FormatFloat("", re.Process.OpenFDs)},
		{"CPU Seconds Total", humanize.FormatFloat("", re.Process.CPUSecondsTotal)},
		{"Max File Descriptors", humanize.FormatFloat("", re.Process.MaxFDs)},
	}

	t3 := widgets.NewTable()
	t3.Title = "Go Runtime Stats"
	t3.TitleStyle = ui.NewStyle(ui.ColorCyan)
	t3.ColumnWidths = []int{30, 50000}
	t3.RowSeparator = false
	t3.Rows = [][]string{
		{"Allocated Memory", humanize.Bytes(uint64(re.Go.AllocBytes))},
		{"Goroutines", humanize.FormatFloat("", re.Go.Goroutines)},
		{"Threads", humanize.FormatFloat("", re.Go.Threads)},
		{"Heap Objects", humanize.FormatFloat("", re.Go.HeapObjects)},
		{"Heap Allocated Memory", humanize.Bytes(uint64(re.Go.HeapAllocBytes))},
		{"Info", re.Go.Info},
	}

	percentileTable := func(title string, hist Histogram) *widgets.Table {
		t := widgets.NewTable()
		t.Title = title
		t.TitleStyle = ui.NewStyle(ui.ColorCyan)

		if hist.Count == 0 {
			t.Rows = [][]string{
				{"No data"},
			}
			return t
		}

		t.ColumnWidths = []int{30, 50000}
		t.RowSeparator = false
		t.Rows = [][]string{
			{"Percentile", "Latency"},
		}
		for _, p := range hist.AsPercentiles() {
			t.Rows = append(t.Rows, []string{
				humanize.FormatFloat("#,###", p.Percentile) + "%",
				fmt.Sprintf("%v", p.Value),
			})
		}
		return t
	}

	grid := ui.NewGrid()
	grid.SetRect(0, 0, termWidth, termHeight)

	tabpane := widgets.NewTabPane("[1] Common", "[2] Backend Stats", "[3] Cache Stats", "[4] Event Stats")
	tabpane.ActiveTabStyle = ui.NewStyle(ui.ColorCyan, ui.ColorClear, ui.ModifierBold|ui.ModifierUnderline)
	tabpane.InactiveTabStyle = ui.NewStyle(ui.ColorCyan)
	tabpane.Border = false

	switch eventID {
	case "", "1":
		tabpane.ActiveTabIndex = 0
		grid.Set(
			ui.NewRow(0.05,
				ui.NewCol(1.0, tabpane),
			),
			ui.NewRow(0.925,
				ui.NewCol(0.5,
					ui.NewRow(0.3, t1),
					ui.NewRow(0.3, t2),
					ui.NewRow(0.3, t3),
				),
				ui.NewCol(0.5,
					ui.NewRow(0.3, percentileTable("Generate Server Certificates Percentiles", re.Cluster.GenerateRequestsHistogram)),
				),
			),
			ui.NewRow(0.025,
				ui.NewCol(1.0, h),
			),
		)
	case "2":
		tabpane.ActiveTabIndex = 1
		grid.Set(
			ui.NewRow(0.05,
				ui.NewCol(1.0, tabpane),
			),
			ui.NewRow(0.925,
				ui.NewCol(0.5,
					ui.NewRow(1.0, backendRequestsTable("Top Backend Requests", re.Backend)),
				),
				ui.NewCol(0.5,
					ui.NewRow(0.3, percentileTable("Backend Read Percentiles", re.Backend.Read)),
					ui.NewRow(0.3, percentileTable("Backend Batch Read Percentiles", re.Backend.BatchRead)),
					ui.NewRow(0.3, percentileTable("Backend Write Percentiles", re.Backend.Write)),
				),
			),
			ui.NewRow(0.025,
				ui.NewCol(1.0, h),
			),
		)
	case "3":
		tabpane.ActiveTabIndex = 2
		grid.Set(
			ui.NewRow(0.05,
				ui.NewCol(1.0, tabpane),
			),
			ui.NewRow(0.925,
				ui.NewCol(0.5,
					ui.NewRow(1.0, backendRequestsTable("Top Cache Requests", re.Cache)),
				),
				ui.NewCol(0.5,
					ui.NewRow(0.3, percentileTable("Cache Read Percentiles", re.Cache.Read)),
					ui.NewRow(0.3, percentileTable("Cache Batch Read Percentiles", re.Cache.BatchRead)),
					ui.NewRow(0.3, percentileTable("Cache Write Percentiles", re.Cache.Write)),
				),
			),
			ui.NewRow(0.025,
				ui.NewCol(1.0, h),
			),
		)
	case "4":
		tabpane.ActiveTabIndex = 3
		grid.Set(
			ui.NewRow(0.05,
				ui.NewCol(1.0, tabpane),
			),
			ui.NewRow(0.925,
				ui.NewCol(0.5,
					ui.NewRow(1.0, eventsTable(re.Watcher)),
				),
				ui.NewCol(0.5,
					ui.NewRow(0.5, eventsGraph("Events/Sec", re.Watcher.EventsPerSecond)),
					ui.NewRow(0.5, eventsGraph("Bytes/Sec", re.Watcher.BytesPerSecond)),
				),
			),
			ui.NewRow(0.025,
				ui.NewCol(1.0, h),
			),
		)
	}
	ui.Render(grid)
	return nil
}

func (c *TopCommand) fetchAndGenerateReport(ctx context.Context, client *roundtrip.Client, prev *Report) (*Report, error) {
	metrics, err := c.getPrometheusMetrics(ctx, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return generateReport(metrics, prev, *c.refreshPeriod)
}

func (c *TopCommand) getPrometheusMetrics(ctx context.Context, client *roundtrip.Client) (map[string]*dto.MetricFamily, error) {
	re, err := client.Get(ctx, client.Endpoint("metrics"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}
	var parser expfmt.TextParser
	return parser.TextToMetricFamilies(re.Reader())
}

// Report is a report rendered over the data
type Report struct {
	// Version is a report version
	Version string
	// Timestamp is the date when this report has been generated
	Timestamp time.Time
	// Hostname is the hostname of the report
	Hostname string
	// Process contains process stats
	Process ProcessStats
	// Go contains go runtime stats
	Go GoStats
	// Backend is a backend stats
	Backend BackendStats
	// Cache is cache stats
	Cache BackendStats
	// Cluster is cluster stats
	Cluster ClusterStats
	// Watcher is watcher stats
	Watcher *WatcherStats
}

// WatcherStats contains watcher stats
type WatcherStats struct {
	// EventSize is an event size histogram
	EventSize Histogram
	// TopEvents is a collection of resources to their events
	TopEvents map[string]Event
	// EventsPerSecond is the events per sec buffer
	EventsPerSecond *utils.CircularBuffer
	// BytesPerSecond is the bytes per sec buffer
	BytesPerSecond *utils.CircularBuffer
}

// SortedTopEvents returns top events sorted either
// by frequency if frequency is present, or by count, if both
// frequency and count are identical then by name to preserve order
func (b *WatcherStats) SortedTopEvents() []Event {
	out := make([]Event, 0, len(b.TopEvents))
	for _, events := range b.TopEvents {
		out = append(out, events)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].GetFreq() != out[j].GetFreq() {
			return out[i].GetFreq() > out[j].GetFreq()
		}

		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}

		return out[i].Resource < out[j].Resource
	})
	return out
}

// Event is a watcher event stats
type Event struct {
	// Resource is the resource of the event
	Resource string
	// Size is the size of the serialized event
	Size float64
	// Counter maintains the count and the resource frequency
	Counter
}

// AverageSize returns the average size for the event
func (e Event) AverageSize() float64 {
	return e.Size / float64(e.Count)
}

// ProcessStats is a process statistics
type ProcessStats struct {
	// CPUSecondsTotal is a total user and system CPU time spent in seconds.
	CPUSecondsTotal float64
	// MaxFDs is the maximum number of open file descriptors.
	MaxFDs float64
	// OpenFDs is a number of open file descriptors.
	OpenFDs float64
	// ResidentMemoryBytes is a resident memory size in bytes.
	ResidentMemoryBytes float64
	// StartTime is a process start time
	StartTime time.Time
}

// GoStats is stats about go runtime
type GoStats struct {
	// Info is a runtime info (version, etc)
	Info string
	// Threads is a number of OS threads created.
	Threads float64
	// Goroutines is a number of goroutines that currently exist.
	Goroutines float64
	// Number of heap bytes allocated and still in use.
	HeapAllocBytes float64
	// Number of bytes allocated and still in use.
	AllocBytes float64
	// HeapObjects is a number of allocated objects.
	HeapObjects float64
}

// BackendStats contains backend stats
type BackendStats struct {
	// Read is a read latency histogram
	Read Histogram
	// BatchRead is a batch read latency histogram
	BatchRead Histogram
	// Write is a write latency histogram
	Write Histogram
	// BatchWrite is a batch write latency histogram
	BatchWrite Histogram
	// TopRequests is a collection of requests to
	// backend and their counts
	TopRequests map[RequestKey]Request
	// QueueSize is a queue size of the cache watcher
	QueueSize float64
}

// SortedTopRequests returns top requests sorted either
// by frequency if frequency is present, or by count, if both
// frequency and count are identical then by name to preserve order
func (b *BackendStats) SortedTopRequests() []Request {
	out := make([]Request, 0, len(b.TopRequests))
	for _, req := range b.TopRequests {
		out = append(out, req)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].GetFreq() != out[j].GetFreq() {
			return out[i].GetFreq() > out[j].GetFreq()
		}

		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}

		return out[i].Key.Key < out[j].Key.Key
	})
	return out
}

// ClusterStats contains some teleport specific stats
type ClusterStats struct {
	// InteractiveSessions is a number of active sessions.
	InteractiveSessions float64
	// RemoteClusters is a list of remote clusters and their status.
	RemoteClusters []RemoteCluster
	// GenerateRequests is a number of active generate requests
	GenerateRequests float64
	// GenerateRequestsCount is a total number of generate requests
	GenerateRequestsCount Counter
	// GenerateRequestThrottledCount is a total number of throttled generate
	// requests
	GenerateRequestsThrottledCount Counter
	// GenerateRequestsHistogram is a histogram of generate requests latencies
	GenerateRequestsHistogram Histogram
	// ActiveMigrations is a set of active migrations
	ActiveMigrations []string
}

// RemoteCluster is a remote cluster (or local cluster)
// connected to this cluster
type RemoteCluster struct {
	// Name is a cluster name
	Name string
	// Connected is true when cluster is connected
	Connected bool
}

// IsConnected returns user-friendly "connected"
// or "disconnected" cluster status
func (rc RemoteCluster) IsConnected() string {
	if rc.Connected {
		return "connected"
	}
	return "disconnected"
}

// RequestKey is a composite request Key
type RequestKey struct {
	// Range is set when it's a range request
	Range bool
	// Key is a backend key and operation
	Key string
}

// IsRange returns user-friendly "range" if
// request is a range request
func (r RequestKey) IsRange() string {
	if r.Range {
		return "range"
	}
	return ""
}

// Request is a backend request stats
type Request struct {
	// Key is a request key
	Key RequestKey
	// Counter maintains the count and the key access frequency
	Counter
}

// Counter contains count and frequency
type Counter struct {
	// Freq is a key access frequency in requests per second
	Freq *float64
	// Count is a last recorded count
	Count int64
}

// SetFreq sets counter frequency based on the previous value
// and the time period
func (c *Counter) SetFreq(prevCount Counter, period time.Duration) {
	if period == 0 {
		return
	}
	freq := float64(c.Count-prevCount.Count) / float64(period/time.Second)
	c.Freq = &freq
}

// GetFreq returns frequency of the request
func (c Counter) GetFreq() float64 {
	if c.Freq == nil {
		return 0
	}
	return *c.Freq
}

// Histogram is a histogram with buckets
type Histogram struct {
	// Count is a total number of elements counted
	Count int64
	// Sum is sum of all elements counted
	Sum float64
	// Buckets is a list of buckets
	Buckets []Bucket
}

// Percentile is a latency percentile
type Percentile struct {
	// Percentile is a percentile value
	Percentile float64
	// Value is a value of the percentile
	Value time.Duration
}

// AsPercentiles interprets histogram as a bucket of percentiles
// and returns calculated percentiles
func (h Histogram) AsPercentiles() []Percentile {
	if h.Count == 0 {
		return nil
	}
	var percentiles []Percentile
	for _, bucket := range h.Buckets {
		if bucket.Count == 0 {
			continue
		}
		if bucket.Count == h.Count || math.IsInf(bucket.UpperBound, 0) {
			percentiles = append(percentiles, Percentile{
				Percentile: 100,
				Value:      time.Duration(bucket.UpperBound * float64(time.Second)),
			})
			return percentiles
		}
		percentiles = append(percentiles, Percentile{
			Percentile: 100 * (float64(bucket.Count) / float64(h.Count)),
			Value:      time.Duration(bucket.UpperBound * float64(time.Second)),
		})
	}
	return percentiles
}

// Bucket is a histogram bucket
type Bucket struct {
	// Count is a count of elements in the bucket
	Count int64
	// UpperBound is an upper bound of the bucket
	UpperBound float64
}

func generateReport(metrics map[string]*dto.MetricFamily, prev *Report, period time.Duration) (*Report, error) {
	// format top backend requests
	hostname, _ := os.Hostname()
	re := Report{
		Version:   types.V1,
		Timestamp: time.Now().UTC(),
		Hostname:  hostname,
		Backend: BackendStats{
			TopRequests: make(map[RequestKey]Request),
		},
		Cache: BackendStats{
			TopRequests: make(map[RequestKey]Request),
		},
	}

	collectBackendStats := func(component string, stats *BackendStats, prevStats *BackendStats) {
		for _, req := range getRequests(component, metrics[teleport.MetricBackendRequests]) {
			if prev != nil {
				prevReq, ok := prevStats.TopRequests[req.Key]
				if ok {
					// if previous value is set, can calculate req / second
					req.SetFreq(prevReq.Counter, period)
				}
			}
			stats.TopRequests[req.Key] = req
		}
		stats.Read = getHistogram(metrics[teleport.MetricBackendReadHistogram], forLabel(component))
		stats.Write = getHistogram(metrics[teleport.MetricBackendWriteHistogram], forLabel(component))
		stats.BatchRead = getHistogram(metrics[teleport.MetricBackendBatchReadHistogram], forLabel(component))
		stats.BatchWrite = getHistogram(metrics[teleport.MetricBackendBatchWriteHistogram], forLabel(component))
	}

	var stats *BackendStats
	if prev != nil {
		stats = &prev.Backend
	}
	collectBackendStats(teleport.ComponentBackend, &re.Backend, stats)
	if prev != nil {
		stats = &prev.Cache
	} else {
		stats = nil
	}
	collectBackendStats(teleport.ComponentCache, &re.Cache, stats)
	re.Cache.QueueSize = getComponentGaugeValue(teleport.Component(teleport.ComponentAuth, teleport.ComponentCache),
		metrics[teleport.MetricBackendWatcherQueues])

	var watchStats *WatcherStats
	if prev != nil {
		watchStats = prev.Watcher
	}

	re.Watcher = getWatcherStats(metrics, watchStats, period)

	re.Process = ProcessStats{
		CPUSecondsTotal:     getGaugeValue(metrics[teleport.MetricProcessCPUSecondsTotal]),
		MaxFDs:              getGaugeValue(metrics[teleport.MetricProcessMaxFDs]),
		OpenFDs:             getGaugeValue(metrics[teleport.MetricProcessOpenFDs]),
		ResidentMemoryBytes: getGaugeValue(metrics[teleport.MetricProcessResidentMemoryBytes]),
		StartTime:           time.Unix(int64(getGaugeValue(metrics[teleport.MetricProcessStartTimeSeconds])), 0),
	}

	re.Go = GoStats{
		Info:           getLabels(metrics[teleport.MetricGoInfo]),
		Threads:        getGaugeValue(metrics[teleport.MetricGoThreads]),
		Goroutines:     getGaugeValue(metrics[teleport.MetricGoGoroutines]),
		AllocBytes:     getGaugeValue(metrics[teleport.MetricGoAllocBytes]),
		HeapAllocBytes: getGaugeValue(metrics[teleport.MetricGoHeapAllocBytes]),
		HeapObjects:    getGaugeValue(metrics[teleport.MetricGoHeapObjects]),
	}

	re.Cluster = ClusterStats{
		InteractiveSessions:            getGaugeValue(metrics[teleport.MetricServerInteractiveSessions]),
		RemoteClusters:                 getRemoteClusters(metrics[teleport.MetricRemoteClusters]),
		GenerateRequests:               getGaugeValue(metrics[teleport.MetricGenerateRequestsCurrent]),
		GenerateRequestsCount:          Counter{Count: getCounterValue(metrics[teleport.MetricGenerateRequests])},
		GenerateRequestsThrottledCount: Counter{Count: getCounterValue(metrics[teleport.MetricGenerateRequestsThrottled])},
		GenerateRequestsHistogram:      getHistogram(metrics[teleport.MetricGenerateRequestsHistogram], atIndex(0)),
		ActiveMigrations:               getActiveMigrations(metrics[prometheus.BuildFQName(teleport.MetricNamespace, "", teleport.MetricMigrations)]),
	}

	if prev != nil {
		re.Cluster.GenerateRequestsCount.SetFreq(prev.Cluster.GenerateRequestsCount, period)
		re.Cluster.GenerateRequestsThrottledCount.SetFreq(prev.Cluster.GenerateRequestsThrottledCount, period)
	}

	return &re, nil
}

// matchesLabelValue returns true if a list of label pairs
// matches required name/value pair, used to slice vectors by component
func matchesLabelValue(labels []*dto.LabelPair, name, value string) bool {
	for _, label := range labels {
		if label.GetName() == name {
			return label.GetValue() == value
		}
	}
	return false
}

func getRequests(component string, metric *dto.MetricFamily) []Request {
	if metric == nil || metric.GetType() != dto.MetricType_COUNTER || len(metric.Metric) == 0 {
		return nil
	}
	out := make([]Request, 0, len(metric.Metric))
	for _, counter := range metric.Metric {
		if !matchesLabelValue(counter.Label, teleport.ComponentLabel, component) {
			continue
		}
		req := Request{
			Counter: Counter{
				Count: int64(*counter.Counter.Value),
			},
		}
		for _, label := range counter.Label {
			if label.GetName() == teleport.TagReq {
				req.Key.Key = label.GetValue()
			}
			if label.GetName() == teleport.TagRange {
				req.Key.Range = label.GetValue() == teleport.TagTrue
			}
		}
		out = append(out, req)
	}
	return out
}

func getWatcherStats(metrics map[string]*dto.MetricFamily, prev *WatcherStats, period time.Duration) *WatcherStats {
	eventsEmitted := metrics[teleport.MetricWatcherEventsEmitted]
	if eventsEmitted == nil || eventsEmitted.GetType() != dto.MetricType_HISTOGRAM || len(eventsEmitted.Metric) == 0 {
		eventsEmitted = &dto.MetricFamily{}
	}

	events := make(map[string]Event)
	for i, metric := range eventsEmitted.Metric {
		histogram := getHistogram(eventsEmitted, atIndex(i))

		resource := ""
		for _, pair := range metric.GetLabel() {
			if pair.GetName() == teleport.TagResource {
				resource = pair.GetValue()
				break
			}
		}

		// only continue processing if we found the resource
		if resource == "" {
			continue
		}

		evt := Event{
			Resource: resource,
			Size:     histogram.Sum,
			Counter: Counter{
				Count: histogram.Count,
			},
		}

		if prev != nil {
			prevReq, ok := prev.TopEvents[evt.Resource]
			if ok {
				// if previous value is set, can calculate req / second
				evt.SetFreq(prevReq.Counter, period)
			}
		}

		events[evt.Resource] = evt
	}

	histogram := getHistogram(metrics[teleport.MetricWatcherEventSizes], atIndex(0))
	var (
		eventsPerSec *utils.CircularBuffer
		bytesPerSec  *utils.CircularBuffer
	)
	if prev == nil {
		eps, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		bps, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		eventsPerSec = eps
		bytesPerSec = bps
	} else {
		eventsPerSec = prev.EventsPerSecond
		bytesPerSec = prev.BytesPerSecond

		eventsPerSec.Add(float64(histogram.Count-prev.EventSize.Count) / float64(period/time.Second))
		bytesPerSec.Add(histogram.Sum - prev.EventSize.Sum/float64(period/time.Second))
	}

	stats := &WatcherStats{
		EventSize:       histogram,
		TopEvents:       events,
		EventsPerSecond: eventsPerSec,
		BytesPerSecond:  bytesPerSec,
	}

	return stats
}

func getRemoteClusters(metric *dto.MetricFamily) []RemoteCluster {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 {
		return nil
	}
	out := make([]RemoteCluster, len(metric.Metric))
	for i, counter := range metric.Metric {
		rc := RemoteCluster{
			Connected: counter.Gauge.GetValue() > 0,
		}
		for _, label := range counter.Label {
			if label.GetName() == teleport.TagCluster {
				rc.Name = label.GetValue()
			}
		}
		out[i] = rc
	}
	return out
}

func getActiveMigrations(metric *dto.MetricFamily) []string {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 {
		return nil
	}
	var out []string
	for _, counter := range metric.Metric {
		if counter.Gauge.GetValue() == 0 {
			continue
		}
		for _, label := range counter.Label {
			if label.GetName() == teleport.TagMigration {
				out = append(out, label.GetValue())
				break
			}
		}
	}
	return out
}

func getComponentGaugeValue(component string, metric *dto.MetricFamily) float64 {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 || metric.Metric[0].Gauge == nil || metric.Metric[0].Gauge.Value == nil {
		return 0
	}
	for i := range metric.Metric {
		if matchesLabelValue(metric.Metric[i].Label, teleport.ComponentLabel, component) {
			return *metric.Metric[i].Gauge.Value
		}
	}
	return 0
}

func getGaugeValue(metric *dto.MetricFamily) float64 {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 || metric.Metric[0].Gauge == nil || metric.Metric[0].Gauge.Value == nil {
		return 0
	}
	return *metric.Metric[0].Gauge.Value
}

func getCounterValue(metric *dto.MetricFamily) int64 {
	if metric == nil || metric.GetType() != dto.MetricType_COUNTER || len(metric.Metric) == 0 || metric.Metric[0].Counter == nil || metric.Metric[0].Counter.Value == nil {
		return 0
	}
	return int64(*metric.Metric[0].Counter.Value)
}

type histogramFilterFunc func(metrics []*dto.Metric) *dto.Histogram

func atIndex(index int) histogramFilterFunc {
	return func(metrics []*dto.Metric) *dto.Histogram {
		if index < 0 || index >= len(metrics) {
			return nil
		}

		return metrics[index].Histogram
	}
}

func forLabel(label string) histogramFilterFunc {
	return func(metrics []*dto.Metric) *dto.Histogram {
		var hist *dto.Histogram
		for i := range metrics {
			if matchesLabelValue(metrics[i].Label, teleport.ComponentLabel, label) {
				hist = metrics[i].Histogram
				break
			}
		}

		return hist
	}
}

func getHistogram(metric *dto.MetricFamily, filterFn histogramFilterFunc) Histogram {
	if metric == nil || metric.GetType() != dto.MetricType_HISTOGRAM || len(metric.Metric) == 0 || metric.Metric[0].Histogram == nil {
		return Histogram{}
	}

	hist := filterFn(metric.Metric)
	if hist == nil {
		return Histogram{}
	}

	out := Histogram{
		Count:   int64(hist.GetSampleCount()),
		Sum:     hist.GetSampleSum(),
		Buckets: make([]Bucket, len(hist.Bucket)),
	}

	for i, bucket := range hist.Bucket {
		out.Buckets[i] = Bucket{
			Count:      int64(bucket.GetCumulativeCount()),
			UpperBound: bucket.GetUpperBound(),
		}
	}
	return out
}

func getLabels(metric *dto.MetricFamily) string {
	if metric == nil {
		return ""
	}
	var out []string
	for _, metric := range metric.Metric {
		for _, label := range metric.Label {
			out = append(out, fmt.Sprintf("%v:%v", label.GetName(), label.GetValue()))
		}
	}
	return strings.Join(out, ", ")
}
