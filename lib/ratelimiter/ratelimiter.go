/*
Copyright 2015 Gravitational, Inc.

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
package ratelimiter

import (
	"net/http"
	"sync"
	"time"

	"github.com/alexlyulkov/oxy/connlimit"
	"github.com/alexlyulkov/oxy/ratelimit"
	"github.com/alexlyulkov/oxy/utils"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	"github.com/mailgun/ttlmap"
)

type RateLimiter struct {
	*connlimit.ConnLimiter
	tokenLimiter *ratelimit.TokenLimiter
	rateLimits   *ttlmap.TtlMap
	*sync.Mutex
	rates          *ratelimit.RateSet
	connections    map[string]int64
	maxConnections int64
}

type Rate struct {
	Period  time.Duration
	Average int64
	Burst   int64
}

type RateLimiterConfig struct {
	Rates            []Rate
	MaxConnections   int64 `yaml:"max_connections"`
	MaxNumberOfUsers int   `yaml:"max_users"`
}

func NewRateLimiter(config RateLimiterConfig) (*RateLimiter, error) {
	limiter := RateLimiter{
		Mutex:          &sync.Mutex{},
		maxConnections: config.MaxConnections,
		connections:    make(map[string]int64),
	}

	ipExtractor, err := utils.NewExtractor("client.ip")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	limiter.ConnLimiter, err = connlimit.New(nil, ipExtractor, config.MaxConnections)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	limiter.rates = ratelimit.NewRateSet()
	for _, rate := range config.Rates {
		limiter.rates.Add(rate.Period, rate.Average, rate.Burst)
	}
	if len(config.Rates) == 0 {
		limiter.rates.Add(time.Second, DefaultRate, DefaultRate)
	}

	limiter.tokenLimiter, err = ratelimit.New(nil, ipExtractor,
		limiter.rates)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	maxNumberOfUsers := config.MaxNumberOfUsers
	if maxNumberOfUsers <= 0 {
		maxNumberOfUsers = DefaultMaxNumberOfUsers
	}
	limiter.rateLimits, err = ttlmap.NewMap(maxNumberOfUsers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &limiter, nil
}

func (l *RateLimiter) Consume(token string, amount int64) error {
	l.Lock()
	defer l.Unlock()

	bucketSetI, exists := l.rateLimits.Get(token)
	var bucketSet *ratelimit.TokenBucketSet

	if exists {
		bucketSet = bucketSetI.(*ratelimit.TokenBucketSet)
		bucketSet.Update(l.rates)
	} else {
		bucketSet = ratelimit.NewTokenBucketSet(l.rates, &timetools.RealTime{})
		// We set ttl as 10 times rate period. E.g. if rate is 100 requests/second per client ip
		// the counters for this ip will expire after 10 seconds of inactivity
		l.rateLimits.Set(token, bucketSet, int(bucketSet.GetMaxPeriod()/time.Second)*10+1)
	}
	delay, err := bucketSet.Consume(amount)
	if err != nil {
		return err
	}
	if delay > 0 {
		return &ratelimit.MaxRateError{}
	}
	return nil
}

// Add connection limiter to the handle
func (l *RateLimiter) WrapHTTP(h http.Handler) {
	l.tokenLimiter.Wrap(h)
	l.ConnLimiter.Wrap(l.tokenLimiter)
}

func (l *RateLimiter) AcquireConnection(token string) error {
	if l.maxConnections == 0 {
		return nil
	}

	l.Lock()
	defer l.Unlock()

	numberOfConnections, exists := l.connections[token]
	if !exists {
		l.connections[token] = 1
		return nil
	} else {
		if numberOfConnections >= l.maxConnections {
			return trace.Errorf("Too many connections from %v", token)
		}
		l.connections[token] = numberOfConnections + 1
		return nil
	}
}

func (l *RateLimiter) ReleaseConnection(token string) {
	if l.maxConnections == 0 {
		return
	}

	l.Lock()
	defer l.Unlock()

	numberOfConnections, exists := l.connections[token]
	if !exists {
		log.Errorf("Trying to set negative number of connections")
	} else {
		if numberOfConnections <= 1 {
			delete(l.connections, token)
		} else {
			l.connections[token] = numberOfConnections - 1
		}
	}
}

const (
	DefaultMaxNumberOfUsers = 100000
	DefaultRate             = 100000000
)
