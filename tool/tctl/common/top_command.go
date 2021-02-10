/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

	"github.com/gravitational/teleport"
	auth "github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"

	"github.com/dustin/go-humanize"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// TopCommand implements `tctl token` group of commands.
type TopCommand struct {
	config *service.Config

	// CLI clauses (subcommands)
	top           *kingpin.CmdClause
	diagURL       *string
	refreshPeriod *time.Duration
}

// Initialize allows TopCommand to plug itself into the CLI parser.
func (c *TopCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	c.top = app.Command("top", "Report diagnostic information")
	c.diagURL = c.top.Arg("diag-addr", "Diagnostic HTTP URL").Default("http://127.0.0.1:3000").String()
	c.refreshPeriod = c.top.Arg("refresh", "Refresh period").Default("5s").Duration()
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *TopCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.top.FullCommand():
		diagClient, err := roundtrip.NewClient(*c.diagURL, "")
		if err != nil {
			return true, trace.Wrap(err)
		}
		err = c.Top(diagClient)
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
func (c *TopCommand) Top(client *roundtrip.Client) error {
	if err := ui.Init(); err != nil {
		return trace.Wrap(err)
	}
	defer ui.Close()

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

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
			if e.ID == "1" || e.ID == "2" || e.ID == "3" {
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
		re.Timestamp.Format(teleport.HumanDateFormatSeconds), re.Hostname)
	h.Border = false
	h.TextStyle = ui.NewStyle(ui.ColorMagenta)

	backendRequestsTable := func(title string, b BackendStats) *widgets.Table {
		t := widgets.NewTable()
		t.Title = title
		t.TitleStyle = ui.NewStyle(ui.ColorCyan)
		t.ColumnWidths = []int{10, 10, 10, 50000}
		t.RowSeparator = false
		t.Rows = [][]string{
			[]string{"Count", "Req/Sec", "Range", "Key"},
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

	t1 := widgets.NewTable()
	t1.Title = "Cluster Stats"
	t1.TitleStyle = ui.NewStyle(ui.ColorCyan)
	t1.ColumnWidths = []int{30, 50000}
	t1.RowSeparator = false
	t1.Rows = [][]string{
		[]string{"Interactive Sessions", humanize.FormatFloat("", re.Cluster.InteractiveSessions)},
		[]string{"Cert Gen Active Requests", humanize.FormatFloat("", re.Cluster.GenerateRequests)},
		[]string{"Cert Gen Requests/sec", humanize.FormatFloat("", re.Cluster.GenerateRequestsCount.GetFreq())},
		[]string{"Cert Gen Throttled Requests/sec", humanize.FormatFloat("", re.Cluster.GenerateRequestsThrottledCount.GetFreq())},
		[]string{"Auth Watcher Queue Size", humanize.FormatFloat("", re.Cache.QueueSize)},
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
		[]string{"Start Time", re.Process.StartTime.Format(teleport.HumanDateFormatSeconds)},
		[]string{"Resident Memory Bytes", humanize.Bytes(uint64(re.Process.ResidentMemoryBytes))},
		[]string{"Open File Descriptors", humanize.FormatFloat("", re.Process.OpenFDs)},
		[]string{"CPU Seconds Total", humanize.FormatFloat("", re.Process.CPUSecondsTotal)},
		[]string{"Max File Descriptors", humanize.FormatFloat("", re.Process.MaxFDs)},
	}

	t3 := widgets.NewTable()
	t3.Title = "Go Runtime Stats"
	t3.TitleStyle = ui.NewStyle(ui.ColorCyan)
	t3.ColumnWidths = []int{30, 50000}
	t3.RowSeparator = false
	t3.Rows = [][]string{
		[]string{"Allocated Memory", humanize.Bytes(uint64(re.Go.AllocBytes))},
		[]string{"Goroutines", humanize.FormatFloat("", re.Go.Goroutines)},
		[]string{"Threads", humanize.FormatFloat("", re.Go.Threads)},
		[]string{"Heap Objects", humanize.FormatFloat("", re.Go.HeapObjects)},
		[]string{"Heap Allocated Memory", humanize.Bytes(uint64(re.Go.HeapAllocBytes))},
		[]string{"Info", re.Go.Info},
	}

	percentileTable := func(title string, hist Histogram) *widgets.Table {
		t := widgets.NewTable()
		t.Title = title
		t.TitleStyle = ui.NewStyle(ui.ColorCyan)

		if hist.Count == 0 {
			t.Rows = [][]string{
				[]string{"No data"},
			}
			return t
		}

		t.ColumnWidths = []int{30, 50000}
		t.RowSeparator = false
		t.Rows = [][]string{
			[]string{"Percentile", "Latency"},
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
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)

	tabpane := widgets.NewTabPane("[1] Common", "[2] Backend Stats", "[3] Cache Stats")
	tabpane.ActiveTabStyle = ui.NewStyle(ui.ColorCyan, ui.ColorClear, ui.ModifierBold|ui.ModifierUnderline)
	tabpane.InactiveTabStyle = ui.NewStyle(ui.ColorCyan)
	tabpane.Border = false

	switch eventID {
	case "", "1":
		tabpane.ActiveTabIndex = 0
		grid.Set(
			ui.NewRow(0.05,
				ui.NewCol(0.3, tabpane),
				ui.NewCol(0.7, h),
			),
			ui.NewRow(0.95,
				ui.NewCol(0.5,
					ui.NewRow(0.3, t1),
					ui.NewRow(0.3, t2),
					ui.NewRow(0.3, t3),
				),
				ui.NewCol(0.5,
					ui.NewRow(0.3, percentileTable("Generate Server Certificates Histogram", re.Cluster.GenerateRequestsHistogram)),
				),
			),
		)
	case "2":
		tabpane.ActiveTabIndex = 1
		grid.Set(
			ui.NewRow(0.05,
				ui.NewCol(0.3, tabpane),
				ui.NewCol(0.7, h),
			),
			ui.NewRow(0.95,
				ui.NewCol(0.5,
					ui.NewRow(1.0, backendRequestsTable("Top Backend Requests", re.Backend)),
				),
				ui.NewCol(0.5,
					ui.NewRow(0.3, percentileTable("Backend Read Percentiles", re.Backend.Read)),
					ui.NewRow(0.3, percentileTable("Backend Batch Read Percentiles", re.Backend.BatchRead)),
					ui.NewRow(0.3, percentileTable("Backend Write Percentiles", re.Backend.Write)),
				),
			),
		)
	case "3":
		tabpane.ActiveTabIndex = 2
		grid.Set(
			ui.NewRow(0.05,
				ui.NewCol(0.3, tabpane),
				ui.NewCol(0.7, h),
			),
			ui.NewRow(0.95,
				ui.NewCol(0.5,
					ui.NewRow(1.0, backendRequestsTable("Top Cache Requests", re.Cache)),
				),
				ui.NewCol(0.5,
					ui.NewRow(0.3, percentileTable("Cache Read Percentiles", re.Cache.Read)),
					ui.NewRow(0.3, percentileTable("Cache Batch Read Percentiles", re.Cache.BatchRead)),
					ui.NewRow(0.3, percentileTable("Cache Write Percentiles", re.Cache.Write)),
				),
			),
		)
	}
	ui.Render(grid)
	return nil
}

func (c *TopCommand) fetchAndGenerateReport(ctx context.Context, client *roundtrip.Client, prev *Report) (*Report, error) {
	metrics, err := c.getPrometheusMetrics(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return generateReport(metrics, prev, *c.refreshPeriod)
}

func (c *TopCommand) getPrometheusMetrics(client *roundtrip.Client) (map[string]*dto.MetricFamily, error) {
	re, err := client.Get(context.TODO(), client.Endpoint("metrics"), url.Values{})
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
	//HeapObjects is a number of allocated objects.
	HeapObjects float64
}

// BackendStats contains backend stats
type BackendStats struct {
	// Read is a read latency historgram
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
// by frequency if frequency is present, or by count otherwise
func (b *BackendStats) SortedTopRequests() []Request {
	out := make([]Request, 0, len(b.TopRequests))
	for _, req := range b.TopRequests {
		out = append(out, req)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GetFreq() == out[j].GetFreq() {
			return out[i].Count > out[j].Count
		}
		return out[i].GetFreq() > out[j].GetFreq()
	})
	return out
}

// ClusterStats contains some teleport specifc stats
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
	// Freq is a key access frequency
	Freq *float64
	// Count is a last recorded count
	Count int64
}

// GetFreq returns frequency of the request
func (r Request) GetFreq() float64 {
	if r.Freq == nil {
		return 0
	}
	return *r.Freq
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

// AsPercentiles interprets historgram as a bucket of percentiles
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
		Version:   services.V1,
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
					freq := float64(req.Count-prevReq.Count) / float64(period/time.Second)
					req.Freq = &freq
				}
			}
			stats.TopRequests[req.Key] = req
		}
		stats.Read = getComponentHistogram(component, metrics[teleport.MetricBackendReadHistogram])
		stats.Write = getComponentHistogram(component, metrics[teleport.MetricBackendWriteHistogram])
		stats.BatchRead = getComponentHistogram(component, metrics[teleport.MetricBackendBatchReadHistogram])
		stats.BatchWrite = getComponentHistogram(component, metrics[teleport.MetricBackendBatchWriteHistogram])
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
		GenerateRequestsHistogram:      getHistogram(metrics[teleport.MetricGenerateRequestsHistogram]),
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
			Count: int64(*counter.Counter.Value),
		}
		for _, label := range counter.Label {
			if label.GetName() == teleport.TagReq {
				req.Key.Key = label.GetValue()
			}
			if label.GetName() == teleport.TagRange {
				req.Key.Range = (label.GetValue() == teleport.TagTrue)
			}
		}
		out = append(out, req)
	}
	return out
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

func getComponentHistogram(component string, metric *dto.MetricFamily) Histogram {
	if metric == nil || metric.GetType() != dto.MetricType_HISTOGRAM || len(metric.Metric) == 0 || metric.Metric[0].Histogram == nil {
		return Histogram{}
	}
	var hist *dto.Histogram
	for i := range metric.Metric {
		if matchesLabelValue(metric.Metric[i].Label, teleport.ComponentLabel, component) {
			hist = metric.Metric[i].Histogram
			break
		}
	}
	if hist == nil {
		return Histogram{}
	}
	out := Histogram{
		Count: int64(hist.GetSampleCount()),
	}
	for _, bucket := range hist.Bucket {
		out.Buckets = append(out.Buckets, Bucket{
			Count:      int64(bucket.GetCumulativeCount()),
			UpperBound: bucket.GetUpperBound(),
		})
	}
	return out
}

func getHistogram(metric *dto.MetricFamily) Histogram {
	if metric == nil || metric.GetType() != dto.MetricType_HISTOGRAM || len(metric.Metric) == 0 || metric.Metric[0].Histogram == nil {
		return Histogram{}
	}
	hist := metric.Metric[0].Histogram
	out := Histogram{
		Count: int64(hist.GetSampleCount()),
	}
	for _, bucket := range hist.Bucket {
		out.Buckets = append(out.Buckets, Bucket{
			Count:      int64(bucket.GetCumulativeCount()),
			UpperBound: bucket.GetUpperBound(),
		})
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
