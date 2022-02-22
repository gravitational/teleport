// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/status"
)

type reporter interface {
	getRequestCounter() *prometheus.CounterVec
	getHandledCounter() *prometheus.CounterVec
	getStreamMsgReceivedCounter() *prometheus.CounterVec
	getStreamMsgSentCounter() *prometheus.CounterVec
	getHandledHistogram() *prometheus.HistogramVec
	getStreamReceivedHistogram() *prometheus.HistogramVec
	getStreamSentHistogram() *prometheus.HistogramVec
}

// request stores grpc request fields
type request struct {
	service string
	method  string
}

// splitSericeMethod splits a grpc request path into service and method strings
// request format /%s/%s
func splitServiceMethod(req string) request {
	splitter := func(c rune) bool {
		return c == '/'
	}

	res := strings.FieldsFunc(req, splitter)
	if len(res) == 2 {
		return request{
			service: res[0],
			method:  res[1],
		}
	}

	return request{}
}

// statusCode tries to extract a grpc request status code out of an error
func statusCode(err error) string {
	status, _ := status.FromError(err)
	return status.Code().String()
}

// requestReporter is grpc request specific metrics reporter.
type requestReporter struct {
	reporter
	req          request
	startTime    time.Time
	sendTimer    *prometheus.Timer
	receiveTimer *prometheus.Timer
}

// newRequestReporter returns a new requestReporter object. it also starts and reports
// connectivity metrics
func newRequestReporter(rep reporter, req string) *requestReporter {
	rr := &requestReporter{
		reporter:  rep,
		req:       splitServiceMethod(req),
		startTime: time.Now(),
	}

	sendHist := rr.getStreamSentHistogram().WithLabelValues(rr.req.service, rr.req.method)
	rr.sendTimer = prometheus.NewTimer(sendHist)

	receiveHist := rr.getStreamReceivedHistogram().WithLabelValues(rr.req.service, rr.req.method)
	rr.receiveTimer = prometheus.NewTimer(receiveHist)

	rr.getRequestCounter().WithLabelValues(rr.req.service, rr.req.method).Inc()

	return rr
}

// reportCall reports request specific metrics
func (r *requestReporter) reportCall(err error) {
	r.getHandledCounter().WithLabelValues(r.req.service, r.req.method, statusCode(err)).Inc()
	r.getHandledHistogram().WithLabelValues(r.req.service, r.req.method).Observe(time.Since(r.startTime).Seconds())
}

// reportMsgSent reports message sent specific metrics
func (r *requestReporter) reportMsgSent(err error, size int) {
	r.getStreamMsgSentCounter().WithLabelValues(r.req.service, r.req.method, statusCode(err), strconv.Itoa(size)).Inc()
	r.sendTimer.ObserveDuration()
}

// reportMsgReceived reports message received specific metrics
func (r *requestReporter) reportMsgReceived(err error, size int) {
	r.getStreamMsgReceivedCounter().WithLabelValues(r.req.service, r.req.method, statusCode(err), strconv.Itoa(size)).Inc()
	r.receiveTimer.ObserveDuration()
}
