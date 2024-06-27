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

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/labels"
)

const (
	// AWSInstanceStateName represents the state of the AWS EC2
	// instance - (pending | running | shutting-down | terminated | stopping | stopped )
	// https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-instances.html
	// Used for filtering instances for automatic EC2 discovery
	AWSInstanceStateName = "instance-state-name"

	awsEventPrefix = "aws/"
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

	// Integration is the integration used to fetch the Instance and should be used to access it.
	// Might be empty for instances that didn't use an Integration.
	Integration string

	// EnrollMode is the mode used to enroll the instance into Teleport.
	EnrollMode types.InstallParamEnrollMode
}

// EC2Instance represents an AWS EC2 instance that has been
// discovered.
type EC2Instance struct {
	InstanceID       string
	Tags             map[string]string
	OriginalInstance ec2.Instance
}

func toEC2Instance(originalInst *ec2.Instance) EC2Instance {
	inst := EC2Instance{
		InstanceID:       aws.StringValue(originalInst.InstanceId),
		Tags:             make(map[string]string, len(originalInst.Tags)),
		OriginalInstance: *originalInst,
	}
	for _, tag := range originalInst.Tags {
		if key := aws.StringValue(tag.Key); key != "" {
			inst.Tags[key] = aws.StringValue(tag.Value)
		}
	}
	return inst
}

// ToEC2Instances converts aws []*ec2.Instance to []EC2Instance
func ToEC2Instances(insts []*ec2.Instance) []EC2Instance {
	var ec2Insts []EC2Instance

	for _, inst := range insts {
		ec2Insts = append(ec2Insts, toEC2Instance(inst))
	}
	return ec2Insts
}

// ServerInfos creates a ServerInfo resource for each discovered instance.
func (i *EC2Instances) ServerInfos() ([]types.ServerInfo, error) {
	serverInfos := make([]types.ServerInfo, 0, len(i.Instances))
	for _, instance := range i.Instances {
		tags := make(map[string]string, len(instance.Tags))
		for k, v := range instance.Tags {
			tags[labels.FormatCloudLabelKey(labels.AWSLabelNamespace, k)] = v
		}

		si, err := types.NewServerInfo(types.Metadata{
			Name: types.ServerInfoNameFromAWS(i.AccountID, instance.InstanceID),
		}, types.ServerInfoSpecV1{
			NewLabels: tags,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serverInfos = append(serverInfos, si)
	}

	return serverInfos, nil
}

// Option is a functional option for the Watcher.
type Option func(*Watcher)

// WithPollInterval sets the interval at which the watcher will fetch
// instances from AWS.
func WithPollInterval(interval time.Duration) Option {
	return func(w *Watcher) {
		w.pollInterval = interval
	}
}

// MakeEvents generates ResourceCreateEvents for these instances.
func (instances *EC2Instances) MakeEvents() map[string]*usageeventsv1.ResourceCreateEvent {
	resourceType := types.DiscoveredResourceNode

	switch instances.EnrollMode {
	case types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE:
		resourceType = types.DiscoveredResourceEICENode

	case types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT:
		if instances.DocumentName == types.AWSAgentlessInstallerDocument {
			resourceType = types.DiscoveredResourceAgentlessNode
		}
	}

	events := make(map[string]*usageeventsv1.ResourceCreateEvent, len(instances.Instances))
	for _, inst := range instances.Instances {
		events[awsEventPrefix+inst.InstanceID] = &usageeventsv1.ResourceCreateEvent{
			ResourceType:   resourceType,
			ResourceOrigin: types.OriginCloud,
			CloudProvider:  types.CloudAWS,
		}
	}
	return events
}

// NewEC2Watcher creates a new EC2 watcher instance.
func NewEC2Watcher(ctx context.Context, fetchersFn func() []Fetcher, missedRotation <-chan []types.Server, opts ...Option) (*Watcher, error) {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := Watcher{
		fetchersFn:     fetchersFn,
		ctx:            cancelCtx,
		cancel:         cancelFn,
		pollInterval:   time.Minute,
		triggerFetchC:  make(<-chan struct{}),
		InstancesC:     make(chan Instances),
		missedRotation: missedRotation,
	}
	for _, opt := range opts {
		opt(&watcher)
	}
	return &watcher, nil
}

// MatchersToEC2InstanceFetchers converts a list of AWS EC2 Matchers into a list of AWS EC2 Fetchers.
func MatchersToEC2InstanceFetchers(ctx context.Context, matchers []types.AWSMatcher, clients cloud.Clients) ([]Fetcher, error) {
	ret := []Fetcher{}
	for _, matcher := range matchers {
		for _, region := range matcher.Regions {
			// TODO(gavin): support assume_role_arn for ec2.
			ec2Client, err := clients.GetAWSEC2Client(ctx, region,
				cloud.WithCredentialsMaybeIntegration(matcher.Integration),
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			fetcher := newEC2InstanceFetcher(ec2FetcherConfig{
				Matcher:     matcher,
				Region:      region,
				Document:    matcher.SSM.DocumentName,
				EC2Client:   ec2Client,
				Labels:      matcher.Tags,
				Integration: matcher.Integration,
				EnrollMode:  matcher.Params.EnrollMode,
			})
			ret = append(ret, fetcher)
		}
	}
	return ret, nil
}

type ec2FetcherConfig struct {
	Matcher     types.AWSMatcher
	Region      string
	Document    string
	EC2Client   ec2iface.EC2API
	Labels      types.Labels
	Integration string
	EnrollMode  types.InstallParamEnrollMode
}

type ec2InstanceFetcher struct {
	Filters      []*ec2.Filter
	EC2          ec2iface.EC2API
	Region       string
	DocumentName string
	Parameters   map[string]string
	Integration  string
	EnrollMode   types.InstallParamEnrollMode

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
// This is used for limiting the number of instances for API Calls:
// ssm:SendCommand only accepts between 0 and 50.
// ssm:DescribeInstanceInformation only accepts between 5 and 50.
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
		Integration:  cfg.Integration,
		EnrollMode:   cfg.EnrollMode,
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
		Integration:  f.Integration,
	}
	for _, node := range nodes {
		// Heartbeating and expiration keeps Teleport Agents up to date, no need to consider those nodes.
		// Agentless and EICE Nodes don't heartbeat, so they must be manually managed by the DiscoveryService.
		if !types.IsOpenSSHNodeSubKind(node.GetSubKind()) {
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
			Integration:  insts.Integration,
		}
		instColl = append(instColl, Instances{EC2: &inst})
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
						Integration:  f.Integration,
						EnrollMode:   f.EnrollMode,
					}
					for _, ec2inst := range res.Instances[i:end] {
						f.cachedInstances.add(ownerID, aws.StringValue(ec2inst.InstanceId))
					}
					instances = append(instances, Instances{EC2: &inst})
				}
			}
			return true
		})
	if err != nil {
		return nil, awslib.ConvertRequestFailureError(err)
	}

	if len(instances) == 0 {
		return nil, trace.NotFound("no ec2 instances found")
	}

	return instances, nil
}
