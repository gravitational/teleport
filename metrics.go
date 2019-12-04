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

package teleport

const (
	// MetricGenerateRequests counts how many generate server keys requests
	// are issued over time
	MetricGenerateRequests = "auth_generate_requests_total"

	// MetricGenerateRequestsThrottled measures how many generate requests
	// are throttled
	MetricGenerateRequestsThrottled = "auth_generate_requests_throttled_total"

	// MetricGenerateRequestsCurrent measures current in-flight requests
	MetricGenerateRequestsCurrent = "auth_generate_requests"

	// MetricGenerateRequestsHistogram measures generate requests latency
	MetricGenerateRequestsHistogram = "auth_generate_seconds"

	// MetricServerInteractiveSessions measures interactive sessions in flight
	MetricServerInteractiveSessions = "server_interactive_sessions_total"

	// MetricRemoteClusters measures connected remote clusters
	MetricRemoteClusters = "remote_clusters"

	// TagCluster is a metric tag for a cluster
	TagCluster = "cluster"
)

const (
	// MetricProcessCPUSecondsTotal measures CPU seconds consumed by process
	MetricProcessCPUSecondsTotal = "process_cpu_seconds_total"
	// MetricProcessMaxFDs shows maximum amount of file descriptors allowed for the process
	MetricProcessMaxFDs = "process_max_fds"
	// MetricProcessOpenFDs shows process open file descriptors
	MetricProcessOpenFDs = "process_open_fds"
	// MetricProcessResidentMemoryBytes measures bytes consumed by process resident memory
	MetricProcessResidentMemoryBytes = "process_resident_memory_bytes"
	// MetricProcessStartTimeSeconds measures process start time
	MetricProcessStartTimeSeconds = "process_start_time_seconds"
)

const (
	// MetricGoThreads is amount of system threads used by Go runtime
	MetricGoThreads = "go_threads"

	// MetricGoGoroutines measures current number of goroutines
	MetricGoGoroutines = "go_goroutines"

	// MetricGoInfo provides information about Go runtime version
	MetricGoInfo = "go_info"

	// MetricGoAllocBytes measures allocated memory bytes
	MetricGoAllocBytes = "go_memstats_alloc_bytes"

	// MetricGoHeapAllocBytes measures heap bytes allocated by Go runtime
	MetricGoHeapAllocBytes = "go_memstats_heap_alloc_bytes"

	// MetricGoHeapObjects measures count of heap objects created by Go runtime
	MetricGoHeapObjects = "go_memstats_heap_objects"
)

const (
	// MetricBackendWatchers is a metric with backend watchers
	MetricBackendWatchers = "backend_watchers_total"

	// MetricBackendWatcherQueues is a metric with backend watcher queues sizes
	MetricBackendWatcherQueues = "backend_watcher_queues_total"

	// MetricBackendRequests measures count of backend requests
	MetricBackendRequests = "backend_requests"

	// MetricBackendReadHistogram measures histogram of backend read latencies
	MetricBackendReadHistogram = "backend_read_seconds"

	// MetricBackendWriteHistogram measures histogram of backend write latencies
	MetricBackendWriteHistogram = "backend_write_seconds"

	// MetricBackendBatchWriteHistogram measures histogram of backend batch write latencies
	MetricBackendBatchWriteHistogram = "backend_batch_write_seconds"

	// MetricBackendBatchReadHistogram measures histogram of backend batch read latencies
	MetricBackendBatchReadHistogram = "backend_batch_read_seconds"

	// MetricBackendWriteRequests measures backend write requests count
	MetricBackendWriteRequests = "backend_write_requests_total"

	// MetricBackendWriteFailedRequests measures failed backend write requests count
	MetricBackendWriteFailedRequests = "backend_write_requests_failed_total"

	// MetricBackendBatchWriteRequests measures batch backend writes count
	MetricBackendBatchWriteRequests = "backend_batch_write_requests_total"

	// MetricBackendBatchFailedWriteRequests measures failed batch backend requests count
	MetricBackendBatchFailedWriteRequests = "backend_batch_write_requests_failed_total"

	// MetricBackendReadRequests measures backend read requests count
	MetricBackendReadRequests = "backend_read_requests_total"

	// MetricBackendFailedReadRequests measures failed backend read requests count
	MetricBackendFailedReadRequests = "backend_read_requests_failed_total"

	// MetricBackendBatchReadRequests measures batch backend read requests count
	MetricBackendBatchReadRequests = "backend_batch_read_requests_total"

	// MetricBackendBatchFailedReadRequests measures failed backend batch read requests count
	MetricBackendBatchFailedReadRequests = "backend_batch_read_requests_failed_total"

	// MetricLostCommandEvents measures the number of command events that were lost
	MetricLostCommandEvents = "bpf_lost_command_events"

	// MetricLostDiskEvents measures the number of disk events that were lost.
	MetricLostDiskEvents = "bpf_lost_disk_events"

	// MetricLostNetworkEvents measures the number of network events that were lost.
	MetricLostNetworkEvents = "bpf_lost_network_events"

	// TagRange is a tag specifying backend requests
	TagRange = "range"

	// TagReq is a tag specifying backend request type
	TagReq = "req"

	// TagTrue is a tag value to mark true values
	TagTrue = "true"

	// TagFalse is a tag value to mark false values
	TagFalse = "false"
)
