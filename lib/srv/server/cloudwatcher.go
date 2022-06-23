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
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// EC2Instances contains information required to send SSM commands to EC2 instances
type EC2Instances struct {
	// Region is the AWS region where the instances are located.
	Region string
	// Document is the SSM document that should be executed on the EC2
	// instances.
	Document string
	// Parameters are parameters passed to the SSM document.
	Parameters map[string]string
	// AccountID is the AWS account the instances belong to.
	AccountID string
	// Instances is a list of discovered EC2 instances
	Instances []*ec2.Instance
}

type Watcher struct {
	// InstancesC can be used to consume newly discovered EC2 instances
	InstancesC chan EC2Instances

	fetchers      []*ec2InstanceFetcher
	fetchInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
}

func (w *Watcher) Run() {
	ticker := time.NewTicker(w.fetchInterval)
	defer ticker.Stop()
	for {
		for _, fetcher := range w.fetchers {
			inst, err := fetcher.GetEC2Instances(w.ctx)
			if err != nil {
				log.WithError(err).Error("Failed to fetch EC2 instances")
				continue
			}
			if inst == nil {
				continue
			}
			select {
			case w.InstancesC <- *inst:
			case <-w.ctx.Done():
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

func (w *Watcher) Stop() {
	w.cancel()
}

func NewCloudServerWatcher(ctx context.Context, matchers []services.AWSMatcher, clients cloud.Clients) (*Watcher, error) {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := Watcher{
		fetchers:      []*ec2InstanceFetcher{},
		ctx:           cancelCtx,
		cancel:        cancelFn,
		fetchInterval: time.Minute,
		InstancesC:    make(chan EC2Instances),
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
				Document:  matcher.SSM.Document,
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
	VPC       string
}

type ec2InstanceFetcher struct {
	Filters    []*ec2.Filter
	EC2        ec2iface.EC2API
	Region     string
	Document   string
	Parameters map[string]string
}

func newEC2InstanceFetcher(cfg ec2FetcherConfig) *ec2InstanceFetcher {
	tagFilters := []*ec2.Filter{&ec2.Filter{
		Name:   aws.String(constants.AWSInstanceStateName),
		Values: aws.StringSlice([]string{ec2.InstanceStateNameRunning}),
	}}
	if cfg.VPC != "" {
		tagFilters = append(tagFilters, &ec2.Filter{
			Name:   aws.String(constants.AWSInstanceStateName),
			Values: aws.StringSlice([]string{cfg.VPC}),
		})
	}

	for key, val := range cfg.Labels {
		tagFilters = append(tagFilters, &ec2.Filter{
			Name:   aws.String("tag:" + key),
			Values: aws.StringSlice(val),
		})
	}
	fetcherConfig := ec2InstanceFetcher{
		EC2:      cfg.EC2Client,
		Filters:  tagFilters,
		Region:   cfg.Region,
		Document: cfg.Document,
		Parameters: map[string]string{
			"token": cfg.Matcher.Params.JoinToken,
		},
	}
	return &fetcherConfig
}

func (f *ec2InstanceFetcher) GetEC2Instances(ctx context.Context) (*EC2Instances, error) {
	var instances []*ec2.Instance
	var accountID string
	err := f.EC2.DescribeInstancesPagesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: f.Filters,
	},
		func(dio *ec2.DescribeInstancesOutput, b bool) bool {
			for _, res := range dio.Reservations {
				if accountID == "" {
					accountID = aws.StringValue(res.OwnerId)
				}
				instances = append(instances, res.Instances...)
			}
			return true
		})

	if err != nil {
		return nil, common.ConvertError(err)
	}

	if len(instances) == 0 {
		return nil, nil
	}

	return &EC2Instances{
		AccountID:  accountID,
		Region:     f.Region,
		Document:   f.Document,
		Instances:  instances,
		Parameters: f.Parameters,
	}, nil
}
