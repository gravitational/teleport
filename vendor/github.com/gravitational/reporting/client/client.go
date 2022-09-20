/*
Copyright 2017 Gravitational, Inc.

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

package client

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/gravitational/reporting"
	"github.com/gravitational/reporting/types"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	grpcapi "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ClientConfig defines the reporting client config
type ClientConfig struct {
	// ServerAddr is the address of the reporting gRPC server
	ServerAddr string
	// ServerName is the SNI server name
	ServerName string
	// Certificate is the client certificate to authenticate with
	Certificate tls.Certificate
	// Insecure is whether the client should skip server cert verification
	Insecure bool
}

// Client defines the reporting client interface
type Client interface {
	// Record records an event
	Record(types.Event)
}

// NewClient returns a new reporting gRPC client
func NewClient(ctx context.Context, config ClientConfig) (*client, error) {
	conn, err := grpcapi.Dial(config.ServerAddr,
		grpcapi.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				ServerName:         config.ServerName,
				InsecureSkipVerify: config.Insecure,
				Certificates:       []tls.Certificate{config.Certificate},
			})))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := &client{
		client: reporting.NewEventsServiceClient(conn),
		// give an extra room to the events channel in case events
		// are generated faster we can flush them (unlikely due to
		// our events nature)
		eventsCh: make(chan types.Event, 5*flushCount),
		ctx:      ctx,
	}
	go client.receiveAndFlushEvents()
	return client, nil
}

type client struct {
	client reporting.EventsServiceClient
	// eventsCh is the channel where events are submitted before they are
	// put into internal buffer
	eventsCh chan types.Event
	// events is the internal events buffer that gets flushed periodically
	events []types.Event
	// ctx may be used to stop client goroutine
	ctx context.Context
}

// Record records an event. Note that the client accumulates events in memory
// and flushes them every once in a while
func (c *client) Record(event types.Event) {
	select {
	case c.eventsCh <- event:
		log.Debugf("queued %v", event)
	default:
		log.Warnf("events channel is full, discarding %v", event)
	}
}

// receiveAndFlushEvents receives events on a channel, accumulates them in
// memory and flushes them once a certain number has been accumulated, or
// certain amount of time has passed
func (c *client) receiveAndFlushEvents() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case event := <-c.eventsCh:
			if len(c.events) >= flushCount {
				if err := c.flush(); err != nil {
					log.Debugf("Events queue full and failed to flush events, discarding %v: %v.",
						event, err)
					continue
				}
			}
			c.events = append(c.events, event)
		case <-ticker.C:
			if err := c.flush(); err != nil {
				log.Debugf("Failed to flush events: %v.", err)
			}
		case <-c.ctx.Done():
			log.Debug("Reporting client is shutting down.")
			if err := c.flush(); err != nil {
				log.Debugf("Failed to flush events: %v.", err)
			}
			return
		}
	}
}

// flush flushes all accumulated events
func (c *client) flush() error {
	if len(c.events) == 0 {
		return nil // nothing to flush
	}
	var grpcEvents reporting.GRPCEvents
	for _, event := range c.events {
		grpcEvent, err := types.ToGRPCEvent(event)
		if err != nil {
			return trace.Wrap(err)
		}
		grpcEvents.Events = append(
			grpcEvents.Events, grpcEvent)
	}
	// if we fail to flush some events here, they will be retried on
	// the next cycle, we may get duplicates but each event includes
	// a unique ID which server sinks can use to de-duplicate
	if _, err := c.client.Record(c.ctx, &grpcEvents); err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("flushed %v events", len(c.events))
	c.events = []types.Event{}
	return nil
}

const (
	// flushInterval is how often the client flushes accumulated events
	flushInterval = 3 * time.Second
	// flushCount is the number of events to accumulate before flush triggers
	flushCount = 5
)
