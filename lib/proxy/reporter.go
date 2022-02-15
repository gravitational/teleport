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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/status"
)

// reporter is a grpc metrics reporting interface.
type reporter interface {
	// reportCall reports metrics related to a request.
	reportCall(err error)

	// reportMsgSent reports outbound metrics in regards to sending a message.
	reportMsgSent(err error)

	// reportMsgReceived reports inbound metrics in regards to receiving a message.
	reportMsgReceived(err error)
}

// clientReporter is an implementation of the reporter interface
// used for reporting grpc client metrics.
type clientReporter struct {
	req          request
	metrics      *clientMetrics
	startTime    time.Time
	sendTimer    *prometheus.Timer
	receiveTimer *prometheus.Timer
}

// newClientReporter returns a new clientReporter object. it also starts and reports
// connectivity metrics
func newClientReporter(req string, metrics *clientMetrics) reporter {
	cr := &clientReporter{
		req:       splitServiceMethod(req),
		metrics:   metrics,
		startTime: time.Now(),
	}

	sendHist := cr.metrics.streamSentHistogram.WithLabelValues(cr.req.service, cr.req.method)
	cr.sendTimer = prometheus.NewTimer(sendHist)

	receiveHist := cr.metrics.streamReceivedHistogram.WithLabelValues(cr.req.service, cr.req.method)
	cr.receiveTimer = prometheus.NewTimer(receiveHist)

	cr.metrics.requestCounter.WithLabelValues(cr.req.service, cr.req.method).Inc()

	return cr
}

// reportCall reports client request specific metrics
func (c *clientReporter) reportCall(err error) {
	c.metrics.handledCounter.WithLabelValues(c.req.service, c.req.method, statusCode(err)).Inc()
	c.metrics.handledHistogram.WithLabelValues(c.req.service, c.req.method).Observe(time.Since(c.startTime).Seconds())
}

// reportMsgSent reports client message sent specific metrics
func (c *clientReporter) reportMsgSent(err error) {
	c.metrics.streamMsgSentCounter.WithLabelValues(c.req.service, c.req.method, statusCode(err)).Inc()
	c.sendTimer.ObserveDuration()
}

// reportMsgReceived reports client message received specific metrics
func (c *clientReporter) reportMsgReceived(err error) {
	c.metrics.streamMsgReceivedCounter.WithLabelValues(c.req.service, c.req.method, statusCode(err)).Inc()
	c.receiveTimer.ObserveDuration()
}

// serverReporter is an implementation of the reporter interface
// used for reporting grpc server metrics
type serverReporter struct {
	req       request
	metrics   *serverMetrics
	startTime time.Time
}

// newServerReporter returns a new serverReporter object. it also starts and reports
// connectivity metrics
func newServerReporter(req string, metrics *serverMetrics) reporter {
	sr := &serverReporter{
		req:       splitServiceMethod(req),
		metrics:   metrics,
		startTime: time.Now(),
	}

	sr.metrics.requestCounter.WithLabelValues(sr.req.service, sr.req.method).Inc()

	return sr
}

// reportCall reports server request specific metrics
func (s *serverReporter) reportCall(err error) {
	s.metrics.handledCounter.WithLabelValues(s.req.service, s.req.method, statusCode(err)).Inc()
	s.metrics.handledHistogram.WithLabelValues(s.req.service, s.req.method).Observe(time.Since(s.startTime).Seconds())
}

// reportMsgSent reports server message sent specific metrics
func (s *serverReporter) reportMsgSent(err error) {
	s.metrics.streamMsgSentCounter.WithLabelValues(s.req.service, s.req.method, statusCode(err)).Inc()
}

// reportMsgReceived reports server message received specific metrics
func (s *serverReporter) reportMsgReceived(err error) {
	s.metrics.streamMsgReceivedCounter.WithLabelValues(s.req.service, s.req.method, statusCode(err)).Inc()
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
