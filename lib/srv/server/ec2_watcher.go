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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
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
	Instances []EC2Instance
	// Rotation is set so instances dont get filtered out for already
	// existing in the teleport instance
	Rotation bool
}

// EC2Instance represents an AWS EC2 instance that has been
// discovered.
type EC2Instance struct {
	InstanceID string
}

func toEC2Instance(inst *ec2.Instance) EC2Instance {
	return EC2Instance{
		InstanceID: aws.StringValue(inst.InstanceId),
	}
}

// ToEC2Instances converts aws []*ec2.Instance to []EC2Instance
func ToEC2Instances(insts []*ec2.Instance) []EC2Instance {
	var ec2Insts []EC2Instance

	for _, inst := range insts {
		ec2Insts = append(ec2Insts, toEC2Instance(inst))
	}
	return ec2Insts

}

// NewEC2Watcher creates a new EC2 watcher instance.
func NewEC2Watcher(ctx context.Context, matchers []types.AWSMatcher, clients cloud.Clients, missedRotation <-chan []types.Server) (*Watcher, error) {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := Watcher{
		fetchers:       []Fetcher{},
		ctx:            cancelCtx,
		cancel:         cancelFn,
		fetchInterval:  time.Minute,
		InstancesC:     make(chan Instances),
		missedRotation: missedRotation,
	}

	for _, matcher := range matchers {
		for _, region := range matcher.Regions {
			// TODO(gavin): support assume_role_arn for ec2.
			ec2Client, err := clients.GetAWSEC2Client(ctx, region)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			fetcher := newEC2InstanceFetcher(ec2FetcherConfig{
				Matcher:   matcher,
				Region:    region,
				Document:  matcher.SSM.DocumentName,
				EC2Client: ec2Client,
				Labels:    matcher.Tags,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			watcher.fetchers = append(watcher.fetchers, fetcher)
		}
	}
	return &watcher, nil
}

type ec2FetcherConfig struct {
	Matcher   types.AWSMatcher
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

	// cachedInstances keeps all of the ec2 instances that were matched
	// in the last run of GetInstances for use as a cache with
	// GetMatchingInstances
	cachedInstances *instancesCache
}

type instancesCache struct {
	sync.Mutex
	instances map[cachedInstanceKey]struct{}
}

func (ic *instancesCache) add(accountID, instanceID string) {
	ic.Lock()
	defer ic.Unlock()
	ic.instances[cachedInstanceKey{accountID: accountID, instanceID: instanceID}] = struct{}{}
}

func (ic *instancesCache) clear() {
	ic.Lock()
	defer ic.Unlock()
	ic.instances = make(map[cachedInstanceKey]struct{})
}

func (ic *instancesCache) exists(accountID, instanceID string) bool {
	ic.Lock()
	defer ic.Unlock()
	_, ok := ic.instances[cachedInstanceKey{accountID: accountID, instanceID: instanceID}]
	return ok
}

type cachedInstanceKey struct {
	accountID  string
	instanceID string
}

const (
	// ParamToken is the name of the invite token parameter sent in the SSM Document
	ParamToken = "token"
	// ParamScriptName is the name of the Teleport install script  sent in the SSM Document
	ParamScriptName = "scriptName"
	// ParamSSHDConfigPath is the path to the OpenSSH config file sent in the SSM Document
	ParamSSHDConfigPath = "sshdConfigPath"
)

// awsEC2APIChunkSize is the max number of instances SSM will send commands to at a time
const awsEC2APIChunkSize = 50

func newEC2InstanceFetcher(cfg ec2FetcherConfig) *ec2InstanceFetcher {
	tagFilters := []*ec2.Filter{{
		Name:   aws.String(AWSInstanceStateName),
		Values: aws.StringSlice([]string{ec2.InstanceStateNameRunning}),
	}}

	if _, ok := cfg.Labels["*"]; !ok {
		for key, val := range cfg.Labels {
			tagFilters = append(tagFilters, &ec2.Filter{
				Name:   aws.String("tag:" + key),
				Values: aws.StringSlice(val),
			})
		}
	} else {
		log.Debug("Not setting any tag filters as there is a '*:...' tag present and AWS doesnt allow globbing on keys")
	}
	var parameters map[string]string
	if cfg.Matcher.Params == nil {
		cfg.Matcher.Params = &types.InstallerParams{}
	}
	if cfg.Matcher.Params.InstallTeleport {
		parameters = map[string]string{
			ParamToken:      cfg.Matcher.Params.JoinToken,
			ParamScriptName: cfg.Matcher.Params.ScriptName,
		}
	} else {
		parameters = map[string]string{
			ParamToken:          cfg.Matcher.Params.JoinToken,
			ParamScriptName:     cfg.Matcher.Params.ScriptName,
			ParamSSHDConfigPath: cfg.Matcher.Params.SSHDConfig,
		}
	}

	fetcherConfig := ec2InstanceFetcher{
		EC2:          cfg.EC2Client,
		Filters:      tagFilters,
		Region:       cfg.Region,
		DocumentName: cfg.Document,
		Parameters:   parameters,
		cachedInstances: &instancesCache{
			instances: map[cachedInstanceKey]struct{}{},
		},
	}
	return &fetcherConfig
}

// GetMatchingInstances returns a list of EC2 instances from a list of matching Teleport nodes
func (f *ec2InstanceFetcher) GetMatchingInstances(nodes []types.Server, rotation bool) ([]Instances, error) {
	insts := EC2Instances{
		Region:       f.Region,
		DocumentName: f.DocumentName,
		Parameters:   f.Parameters,
		Rotation:     rotation,
	}
	for _, node := range nodes {
		if node.GetSubKind() != types.SubKindOpenSSHNode {
			continue
		}
		region, ok := node.GetLabel(types.AWSInstanceRegion)
		if !ok || region != f.Region {
			continue
		}
		instID, ok := node.GetLabel(types.AWSInstanceIDLabel)
		if !ok {
			continue
		}
		accountID, ok := node.GetLabel(types.AWSAccountIDLabel)
		if !ok {
			continue
		}

		if !f.cachedInstances.exists(accountID, instID) {
			continue
		}
		if insts.AccountID == "" {
			insts.AccountID = accountID
		}

		insts.Instances = append(insts.Instances, EC2Instance{
			InstanceID: instID,
		})
	}

	if len(insts.Instances) == 0 {
		return nil, trace.NotFound("no ec2 instances found")
	}

	return chunkInstances(insts), nil
}

func chunkInstances(insts EC2Instances) []Instances {
	var instColl []Instances
	for i := 0; i < len(insts.Instances); i += awsEC2APIChunkSize {
		end := i + awsEC2APIChunkSize
		if end > len(insts.Instances) {
			end = len(insts.Instances)
		}
		inst := EC2Instances{
			AccountID:    insts.AccountID,
			Region:       insts.Region,
			DocumentName: insts.DocumentName,
			Parameters:   insts.Parameters,
			Instances:    insts.Instances[i:end],
			Rotation:     insts.Rotation,
		}
		instColl = append(instColl, Instances{EC2Instances: &inst})
	}
	return instColl
}

// GetInstances fetches all EC2 instances matching configured filters.
func (f *ec2InstanceFetcher) GetInstances(ctx context.Context, rotation bool) ([]Instances, error) {
	var instances []Instances
	f.cachedInstances.clear()
	err := f.EC2.DescribeInstancesPagesWithContext(ctx, &ec2.DescribeInstancesInput{
		Filters: f.Filters,
	},
		func(dio *ec2.DescribeInstancesOutput, b bool) bool {
			for _, res := range dio.Reservations {
				for i := 0; i < len(res.Instances); i += awsEC2APIChunkSize {
					end := i + awsEC2APIChunkSize
					if end > len(res.Instances) {
						end = len(res.Instances)
					}
					ownerID := aws.StringValue(res.OwnerId)
					inst := EC2Instances{
						AccountID:    ownerID,
						Region:       f.Region,
						DocumentName: f.DocumentName,
						Instances:    ToEC2Instances(res.Instances[i:end]),
						Parameters:   f.Parameters,
						Rotation:     rotation,
					}
					for _, ec2inst := range res.Instances[i:end] {
						f.cachedInstances.add(ownerID, aws.StringValue(ec2inst.InstanceId))
					}
					instances = append(instances, Instances{EC2Instances: &inst})
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
