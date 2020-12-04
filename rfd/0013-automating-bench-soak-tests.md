# RFD 13 - Automating `tsh bench` soak tests and data collection
## Purpose
Automate the running of tests and collection of data (profiles and dashboards) for the soak test portion of the test plan.

### Overview
At the moment, `tsh bench` tool is used to manually perform and collect data for soak tests following commands against a node.

Soak Tests    
Run 4hour soak test with a mix of interactive/non-interactive sessions:   
`tsh bench --duration=4h user@teleport-monster-6757d7b487-x226b ls`   
`tsh bench -i --duration=4h user@teleport-monster-6757d7b487-x226b ps uax`

## Technical Details
### Cluster Setup
Cluster setup and load tests will continue to be a manual process at the moment and is out of scope for this RFD.

### Image generation
A flag will be added to `tsh bench` that can be used to query and capture the test output to analyze. The test output will be exported as a snapshot of the dashboard after the given duration plus 20 minutes. 

```
// duration of test will be 4 hours and 20 minutes
tsh bench \
   --duration=4h \
   --output-image=file.png \
   root@ip-172-31-2-191-us-west-2-compute-internal ls
```

The time range to query Grafana API for can be derived from the start time of the test. An additional 10 minutes to the start and end times as derived to capture initial state and ending state. It is beneficial to capture the dashboard before `tsh bench` to know what the baseline is. We want to have 10 minutes captured after the run because at the 10 minute mark is when memory and other metrics will get cleaned up by the heartbeat interval to then give the cluster time to return to its baseline.

### Metric collection  
In addition, `tsh bench` will be updated to collect debugging information, if the optional flag, `--profiles`, is provided. Golang's standard profiler requires that the servers in which you are pulling the profiles from, start the service with `-d` and `--diag-addr` flags. `tsh bench` will capture these profiles at the start, halfway point, and end of the run and write them to disk. It will be made sure that there is a 200 status code when connecting to `--diag-addr` endpoint to start the test. If the status code is not 200, the test will stop and notify the user with an error message, "can't connect to diagnostic address"  

```
// running tsh bench
tsh bench \
   --profiles \
   --duration=4h \
   root@ip-172-31-2-191-us-west-2-compute-internal ls
```

```
// listing the profiles
$ ls
run-start.tar
run-middle.tar
run-end.tar
```
See [collecting debugging information from teleport servers](https://community.gravitational.com/t/collecting-debugging-information-from-teleport-server/123). 

### Additional Metrics
Grafana dashboards should be updated to add the additional metrics that `tctl top` uses and then they can be captured by the below tooling. Specifically, we want to add these metrics to capture for example how the cache performs during testing.

## Open Questions
### Posting to external services
`tsh bench` can be updated to post to external services like GitHub issues or Slack. Do we want to do this?

### Updating Metric Collection
Sometimes Telegraf and InfluxDB are used to collect metrics and we want to migrate to Prometheus.

Teleport [already exposes HTTP endpoint](https://goteleport.com/teleport/docs/metrics-logs-reference/#teleport-prometheus-endpoint) that is compatible with Prometheus. 

