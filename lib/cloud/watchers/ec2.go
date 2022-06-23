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

package watchers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
)

type EC2Instances struct {
	Region    string
	Document  string
	Instances []*ec2.Instance
}

type ec2InstanceFetcher struct {
	Filters  []*ec2.Filter
	EC2      ec2iface.EC2API
	Region   string
	Document string
}

type ec2FetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// MemoryDB is the MemoryDB API client.
	EC2 ec2iface.EC2API
	// Region is the AWS region to query databases in.
	Region string
	// Document is the AWS document that will be executed during installation after discovery
	Document string
}

func newEc2InstanceFetcher(cfg ec2FetcherConfig) (*ec2InstanceFetcher, error) {
	tagFilters := make([]*ec2.Filter, 0, len(cfg.Labels)+1)
	tagFilters = append(tagFilters, &ec2.Filter{
		Name:   aws.String("instance-state-name"),
		Values: aws.StringSlice([]string{ec2.InstanceStateNameRunning}),
	})
	for key, val := range cfg.Labels {
		tagFilters = append(tagFilters, &ec2.Filter{
			Name:   aws.String("tag:" + key),
			Values: aws.StringSlice(val),
		})
	}
	fetcherConfig := ec2InstanceFetcher{
		EC2:      cfg.EC2,
		Filters:  tagFilters,
		Region:   cfg.Region,
		Document: cfg.Document,
	}
	return &fetcherConfig, nil
}

func (f *ec2InstanceFetcher) Get(ctx context.Context) (types.Databases, error) {
	return nil, trace.NotImplemented("ec2 fetcher")
}

func (f *ec2InstanceFetcher) Kind() fetcherKind {
	return ec2Fetcher
}

func (f *ec2InstanceFetcher) GetEC2Instances(ctx context.Context) (*EC2Instances, error) {
	var instances []*ec2.Instance
	err := f.EC2.DescribeInstancesPagesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: f.Filters,
	},
		func(dio *ec2.DescribeInstancesOutput, b bool) bool {
			for _, res := range dio.Reservations {
				instances = append(instances, res.Instances...)
			}
			return true
		})

	if err != nil {
		return nil, common.ConvertError(err)
	}

	return &EC2Instances{
		Region:    f.Region,
		Document:  f.Document,
		Instances: instances,
	}, nil
}
