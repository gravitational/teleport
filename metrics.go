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

	// MetricProxySSHSessions measures sessions in flight on the proxy
	MetricProxySSHSessions = "proxy_ssh_sessions_total"

	// MetricRemoteClusters measures connected remote clusters
	MetricRemoteClusters = "remote_clusters"

	// MetricTrustedClusters counts trusted clusters
	MetricTrustedClusters = "trusted_clusters"

	// MetricClusterNameNotFound counts times a cluster name was not found
	MetricClusterNameNotFound = "cluster_name_not_found_total"

	// MetricFailedLoginAttempts counts failed login attempts
	MetricFailedLoginAttempts = "failed_login_attempts_total"

	// MetricConnectToNodeAttempts counts ssh attempts
	MetricConnectToNodeAttempts = "connect_to_node_attempts_total"

	// MetricFailedConnectToNodeAttempts counts failed ssh attempts
	MetricFailedConnectToNodeAttempts = "failed_connect_to_node_attempts_total"

	// MetricUserMaxConcurrentSessionsHit counts number of times a user exceeded their max concurrent ssh connections
	MetricUserMaxConcurrentSessionsHit = "user_max_concurrent_sessions_hit_total"

	// MetricProxyConnectionLimitHit counts the number of times the proxy connection limit was exceeded
	MetricProxyConnectionLimitHit = "proxy_connection_limit_exceeded_total"

	// MetricUserLoginCount counts user logins
	MetricUserLoginCount = "user_login_total"

	// MetricHeartbeatConnectionsReceived counts heartbeat connections received by auth
	MetricHeartbeatConnectionsReceived = "heartbeat_connections_received_total"

	// MetricCertificateMismatch counts login failures due to certificate mismatch
	MetricCertificateMismatch = "certificate_mismatch_total"

	// MetricHeartbeatsMissed counts the nodes that failed to heartbeat
	MetricHeartbeatsMissed = "heartbeats_missed_total"

	// MetricWatcherEventsEmitted counts watcher events that are emitted
	MetricWatcherEventsEmitted = "watcher_events"

	// MetricWatcherEventSizes measures the size of watcher events that are emitted
	MetricWatcherEventSizes = "watcher_event_sizes"

	// MetricMissingSSHTunnels returns the number of missing SSH tunnels for this proxy.
	MetricMissingSSHTunnels = "proxy_missing_ssh_tunnels"

	// MetricMigrations tracks for each migration if it is active or not.
	MetricMigrations = "migrations"

	// TagMigration is a metric tag for a migration
	TagMigration = "migration"

	// MetricIncompleteSessionUploads returns the number of incomplete session uploads
	MetricIncompleteSessionUploads = "incomplete_session_uploads_total"

	// TagCluster is a metric tag for a cluster
	TagCluster = "cluster"

	// MetricTotalInstances provides an instance count
	MetricTotalInstances = "total_instances"

	// MetricEnrolledInUpgrades provides total number of instances that advertise an upgrader.
	MetricEnrolledInUpgrades = "enrolled_in_upgrades"

	// MetricUpgraderCounts provides instance count per-upgrader.
	MetricUpgraderCounts = "upgrader_counts"

	// TagUpgrader is a metric tag for upgraders.
	TagUpgrader = "upgrader"

	// MetricsAccessRequestsCreated provides total number of created access requests.
	MetricAccessRequestsCreated = "access_requests_created"
	// TagRoles is a number of roles requested as a part of access request.
	TagRoles = "roles"
	// TagResources is a number of resources requested as a part of access request.
	TagResources = "resources"

	// UserCertificatesCreated provides total number of user certificates generated.
	MetricUserCertificatesGenerated = "user_certificates_generated"
	// TagPrivateKeyPolicy is a private key policy associated with a user's certificates.
	TagPrivateKeyPolicy = "private_key_policy"
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

	// MetricBackendWrites tallies all individual backend writes (this is distinct from backend write
	// requests in that bulk writes count as multiple writes).
	MetricBackendWrites = "backend_writes_total"

	// MetricBackendWriteFailedRequests measures failed backend write requests count
	MetricBackendWriteFailedRequests = "backend_write_requests_failed_total"

	// MetricBackendWriteFailedPreconditionRequests measures the portion of failed backend write requests
	// that failed due to a custom precondition (existence, revision, value, etc).
	MetricBackendWriteFailedPreconditionRequests = "backend_write_requests_failed_precondition_total"

	// MetricBackendAtomicWriteRequests measures backend atomic write requests count
	MetricBackendAtomicWriteRequests = "backend_atomic_write_requests_total"

	// MetricBackendAtomicWriteFailedRequests measures failed backend atomic write requests count
	MetricBackendAtomicWriteFailedRequests = "backend_atomic_write_requests_failed_total"

	// MetricBackendAtomicWriteConditionFailed measures the amount of atomic write requests that result in condition failure.
	MetricBackendAtomicWriteConditionFailed = "backend_atomic_write_condition_failed_total"

	// MetricBackendAtomicWriteHistogram measures histogram of backend write latencies
	MetricBackendAtomicWriteHistogram = "backend_atomic_write_seconds"

	// MetricBackendAtomicWriteSize measures the histogram of atomic write batch sizes
	MetricBackendAtomicWriteSize = "backend_atomic_write_size"

	// MetricBackendAtomicWriteContention counts the amount of times atomic writes experience internal retries due to contention.
	MetricBackendAtomicWriteContention = "backend_atomic_write_contention"

	// MetricBackendBatchWriteRequests measures batch backend writes count
	MetricBackendBatchWriteRequests = "backend_batch_write_requests_total"

	// MetricBackendBatchFailedWriteRequests measures failed batch backend requests count
	MetricBackendBatchFailedWriteRequests = "backend_batch_write_requests_failed_total"

	// MetricBackendReadRequests measures backend read requests count
	MetricBackendReadRequests = "backend_read_requests_total"

	// MetricBackendReads tallies all individual backend reads (this is distinct from backend read
	// requests in that bulk reads count as multiple reads).
	MetricBackendReads = "backend_reads_total"

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

	// MetricLostRestrictedEvents measures the number of restricted events that were lost
	MetricLostRestrictedEvents = "bpf_lost_restricted_events"

	// MetricState tracks the state of the teleport process.
	MetricState = "process_state"

	// MetricNamespace defines the teleport prometheus namespace
	MetricNamespace = "teleport"

	// MetricConnectedResources tracks the number and type of resources connected via keepalives
	MetricConnectedResources = "connected_resources"

	// MetricBuildInfo tracks build information
	MetricBuildInfo = "build_info"

	// MetricCacheEventsReceived tracks the total number of events received by a cache
	MetricCacheEventsReceived = "cache_events"

	// MetricStaleCacheEventsReceived tracks the number of stale events received by a cache
	MetricStaleCacheEventsReceived = "cache_stale_events"

	// MetricRegisteredServers tracks the number of Teleport servers that have successfully registered with the Teleport cluster and have not reached the end of their ttl
	MetricRegisteredServers = "registered_servers"

	// MetricRegisteredServersByInstallMethods tracks the number of Teleport servers, and their installation method,
	// that have successfully registered with the Teleport cluster and have not reached the end of their ttl
	MetricRegisteredServersByInstallMethods = "registered_servers_by_install_methods"

	// MetricReverseSSHTunnels defines the number of connected SSH reverse tunnels to the proxy
	MetricReverseSSHTunnels = "reverse_tunnels_connected"

	// MetricHostedPluginStatus tracks the current status
	// (as defined by types.PluginStatus) for a plugin instance
	MetricHostedPluginStatus = "hosted_plugin_status"

	// MetricTeleportServices tracks which services are currently running in the current Teleport Process.
	MetricTeleportServices = "services"

	// TagRange is a tag specifying backend requests
	TagRange = "range"

	// TagReq is a tag specifying backend request type
	TagReq = "req"

	// TagTrue is a tag value to mark true values
	TagTrue = "true"

	// TagFalse is a tag value to mark false values
	TagFalse = "false"

	// TagResource is a tag specifying the resource for an event
	TagResource = "resource"

	// TagVersion is a prometheus label for version of Teleport built
	TagVersion = "version"

	// TagGitref is a prometheus label for the gitref of Teleport built
	TagGitref = "gitref"

	// TagGoVersion is a prometheus label for version of Go used to build Teleport
	TagGoVersion = "goversion"

	// TagCacheComponent is a prometheus label for the cache component
	TagCacheComponent = "cache_component"

	// TagType is a prometheus label for type of resource or tunnel connected
	TagType = "type"

	// TagServer is a prometheus label to indicate what server the metric is tied to
	TagServer = "server"

	// TagClient is a prometheus label to indicate what client the metric is tied to
	TagClient = "client"

	// TagInstallMethods is a prometheus label to indicate what installation methods
	// were used for the agent.
	// This value comes from UpstreamInventoryAgentMetadata (sourced in lib/inventory/metadata.fetchInstallMethods).
	TagInstallMethods = "install_methods"

	// TagServiceName is the prometheus label to indicate what services are running in the current proxy.
	// Those services are monitored using the Supervisor.
	// Only a subset of services are monitored. See [lib/service.metricsServicesRunningMap]
	// Eg, discovery_service
	TagServiceName = "service_name"

	// TagAutomaticUpdates is a prometheus label to indicate whether the instance
	// is enrolled in automatic updates.
	TagAutomaticUpdates = "automatic_updates"
)

const (
	// MetricUsageEventsSubmitted is a count of usage events that have been generated.
	MetricUsageEventsSubmitted = "usage_events_submitted_total"

	// MetricUsageBatches is a count of batches enqueued for submission.
	MetricUsageBatches = "usage_batches_total"

	// MetricUsageEventsRequeued is a count of events that were requeued after a
	// submission failed.
	MetricUsageEventsRequeued = "usage_events_requeued_total"

	// MetricUsageBatchSubmissionDuration is a histogram of durations it took to
	// submit a batch.
	MetricUsageBatchSubmissionDuration = "usage_batch_submission_duration_seconds"

	// MetricUsageBatchesSubmitted is a count of event batches successfully
	// submitted.
	MetricUsageBatchesSubmitted = "usage_batch_submitted_total"

	// MetricUsageBatchesFailed is a count of event batches that failed to
	// submit.
	MetricUsageBatchesFailed = "usage_batch_failed_total"

	// MetricUsageEventsDropped is a count of events dropped due to the
	// submission buffer reaching a length limit.
	MetricUsageEventsDropped = "usage_events_dropped_total"
)

// athena audit log metrics
const (
	// MetricParquetlogConsumerBatchPorcessingDuration is a histogram of durations it
	// took to process single batch of events.
	MetricParquetlogConsumerBatchPorcessingDuration = "audit_parquetlog_batch_processing_seconds"
	// MetricParquetlogConsumerS3FlushDuration is a histogram of durations it took to
	// flush and close parquet files on s3.
	MetricParquetlogConsumerS3FlushDuration = "audit_parquetlog_s3_flush_seconds"
	// MetricParquetlogConsumerDeleteEventsDuration is a histogram of durations it
	// took to delete events from SQS.
	MetricParquetlogConsumerDeleteEventsDuration = "audit_parquetlog_delete_events_seconds"
	// MetricParquetlogConsumerBatchSize is a histogram of sizes of single batch of events.
	MetricParquetlogConsumerBatchSize = "audit_parquetlog_batch_size"
	// MetricParquetlogConsumerBatchCount is a count of number of events in single batch.
	MetricParquetlogConsumerBatchCount = "audit_parquetlog_batch_count"
	// MetricParquetlogConsumerLastProcessedTimestamp is a timestamp of last finished consumer execution.
	MetricParquetlogConsumerLastProcessedTimestamp = "audit_parquetlog_last_processed_timestamp"
	// MetricParquetlogConsumerOldestProcessedMessage is age of oldest processed message.
	MetricParquetlogConsumerOldestProcessedMessage = "audit_parquetlog_age_oldest_processed_message"
	// MetricAthenaConsumerCollectFailed is a count of number of errors received from sqs collect.
	MetricParquetlogConsumerCollectFailed = "audit_parquetlog_errors_from_collect_count"
)
