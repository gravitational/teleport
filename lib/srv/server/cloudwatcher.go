/*
Copyright 2022 Gravitational, Inc.

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

package server

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	// AWSInstanceStateName represents the state of the AWS EC2
	// instance - (pending | running | shutting-down | terminated | stopping | stopped )
	// https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-instances.html
	// Used for filtering instances for automatic EC2 discovery
	AWSInstanceStateName = "instance-state-name"
)

// EC2Instances contains information required to send SSM commands to EC2 instances
type EC2Instances struct {
	// Region is the AWS region where the instances are located.
	Region string
	// DocumentName is the SSM document that should be executed on the EC2
	// instances.
	DocumentName string
	// Parameters are parameters passed to the SSM document.
	Parameters map[string]string
	// AccountID is the AWS account the instances belong to.
	AccountID string
	// Instances is a list of discovered EC2 instances
	Instances []*ec2.Instance
}

// Watcher allows callers to discover AWS instances matching specified filters.
type Watcher struct {
	// InstancesC can be used to consume newly discovered EC2 instances
	InstancesC chan EC2Instances

	fetchers      []*ec2InstanceFetcher
	fetchInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
}

// Run starts the watcher's main watch loop.
func (w *Watcher) Run() {
	ticker := time.NewTicker(w.fetchInterval)
	defer ticker.Stop()
	for {
		for _, fetcher := range w.fetchers {
			instancesColl, err := fetcher.GetEC2Instances(w.ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					continue
				}
				log.WithError(err).Error("Failed to fetch EC2 instances")
				continue
			}
			for _, inst := range instancesColl {
				select {
				case w.InstancesC <- inst:
				case <-w.ctx.Done():
				}
			}
		}
		select {
		case <-ticker.C:
			continue
		case <-w.ctx.Done():
			return
		}
	}
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	w.cancel()
}

// NewCloudWatcher creates a new cloud watcher instance.
func NewCloudWatcher(ctx context.Context, matchers []services.AWSMatcher, clients cloud.Clients) (*Watcher, error) {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := Watcher{
		fetchers:      []*ec2InstanceFetcher{},
		ctx:           cancelCtx,
		cancel:        cancelFn,
		fetchInterval: time.Minute,
		InstancesC:    make(chan EC2Instances, 2),
	}
	for _, matcher := range matchers {
		for _, region := range matcher.Regions {
			cl, err := clients.GetAWSEC2Client(region)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			fetcher := newEC2InstanceFetcher(ec2FetcherConfig{
				Matcher:   matcher,
				Region:    region,
				Document:  matcher.SSM.DocumentName,
				EC2Client: cl,
				Labels:    matcher.Tags,
			})
			watcher.fetchers = append(watcher.fetchers, fetcher)
		}
	}
	return &watcher, nil
}

type ec2FetcherConfig struct {
	Matcher   services.AWSMatcher
	Region    string
	Document  string
	EC2Client ec2iface.EC2API
	Labels    types.Labels
}

type ec2InstanceFetcher struct {
	Filters      []*ec2.Filter
	EC2          ec2iface.EC2API
	Region       string
	DocumentName string
	Parameters   map[string]string
}

func newEC2InstanceFetcher(cfg ec2FetcherConfig) *ec2InstanceFetcher {
	tagFilters := []*ec2.Filter{{
		Name:   aws.String(AWSInstanceStateName),
		Values: aws.StringSlice([]string{ec2.InstanceStateNameRunning}),
	}}

	for key, val := range cfg.Labels {
		tagFilters = append(tagFilters, &ec2.Filter{
			Name:   aws.String("tag:" + key),
			Values: aws.StringSlice(val),
		})
	}
	fetcherConfig := ec2InstanceFetcher{
		EC2:          cfg.EC2Client,
		Filters:      tagFilters,
		Region:       cfg.Region,
		DocumentName: cfg.Document,
		Parameters: map[string]string{
			"token":      cfg.Matcher.Params.JoinToken,
			"scriptName": cfg.Matcher.Params.ScriptName,
		},
	}
	return &fetcherConfig
}

// GetEC2Instances fetches all EC2 instances matching configured filters.
func (f *ec2InstanceFetcher) GetEC2Instances(ctx context.Context) ([]EC2Instances, error) {
	var instances []EC2Instances
	err := f.EC2.DescribeInstancesPagesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: f.Filters,
	},
		func(dio *ec2.DescribeInstancesOutput, b bool) bool {
			const chunkSize = 50 // max number of instances SSM will send commands to at a time
			for _, res := range dio.Reservations {
				for i := 0; i < len(res.Instances); i += chunkSize {
					end := i + chunkSize
					if end > len(res.Instances) {
						end = len(res.Instances)
					}
					inst := EC2Instances{
						AccountID:    aws.StringValue(res.OwnerId),
						Region:       f.Region,
						DocumentName: f.DocumentName,
						Instances:    res.Instances[i:end],
						Parameters:   f.Parameters,
					}
					instances = append(instances, inst)
				}
			}
			return true
		})

	if err != nil {
		return nil, common.ConvertError(err)
	}

	if len(instances) == 0 {
		return nil, trace.NotFound("no ec2 instances found")
	}

	return instances, nil
}
