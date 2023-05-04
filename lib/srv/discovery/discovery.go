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

package discovery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers"
	"github.com/gravitational/teleport/lib/srv/server"
)

// Config provides configuration for the discovery server.
type Config struct {
	// Clients is an interface for retrieving cloud clients.
	Clients cloud.Clients
	// AWSMatchers is a list of AWS EC2 matchers.
	AWSMatchers []services.AWSMatcher
	// AzureMatchers is a list of Azure matchers to discover resources.
	AzureMatchers []services.AzureMatcher
	// GCPMatchers is a list of GCP matchers to discover resources.
	GCPMatchers []services.GCPMatcher
	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter
	// AccessPoint is a discovery access point
	AccessPoint auth.DiscoveryAccessPoint
	// Log is the logger.
	Log logrus.FieldLogger
	// DiscoveryGroup is the name of the discovery group that the current
	// discovery service is a part of.
	// It is used to filter out discovered resources that belong to another
	// discovery services. When running in high availability mode and the agents
	// have access to the same cloud resources, this field value must be the same
	// for all discovery services. If different agents are used to discover different
	// sets of cloud resources, this field must be different for each set of agents.
	DiscoveryGroup string
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Clients == nil {
		c.Clients = cloud.NewClients()
	}
	if len(c.AWSMatchers) == 0 && len(c.AzureMatchers) == 0 && len(c.GCPMatchers) == 0 {
		return trace.BadParameter("no matchers configured for discovery")
	}
	if c.Emitter == nil {
		return trace.BadParameter("no Emitter configured for discovery")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("no AccessPoint configured for discovery")
	}
	if c.Log == nil {
		c.Log = logrus.New()
	}

	c.Log = c.Log.WithField(trace.Component, teleport.ComponentDiscovery)
	return nil
}

// Server is a discovery server, used to discover cloud resources for
// inclusion in Teleport
type Server struct {
	*Config

	ctx context.Context
	// cancelfn is used with ctx when stopping the discovery server
	cancelfn context.CancelFunc
	// nodeWatcher is a node watcher.
	nodeWatcher *services.NodeWatcher

	// cloudWatcher periodically retrieves cloud resources, currently
	// only EC2
	cloudWatcher *server.Watcher
	// ec2Installer is used to start the installation process on discovered EC2 nodes
	ec2Installer *server.SSMInstaller
	// kubeFetchers holds all kubernetes fetchers for Azure and other clouds.
	kubeFetchers []fetchers.Fetcher
}

// New initializes a discovery Server
func New(ctx context.Context, cfg *Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	localCtx, cancelfn := context.WithCancel(ctx)
	s := &Server{
		Config:   cfg,
		ctx:      localCtx,
		cancelfn: cancelfn,
	}

	if err := s.initAWSWatchers(cfg.AWSMatchers); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.initAzureWatchers(ctx, cfg.AzureMatchers); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.initGCPWatchers(ctx, cfg.GCPMatchers); err != nil {
		return nil, trace.Wrap(err)
	}

	if s.cloudWatcher != nil {
		if err := s.initTeleportNodeWatcher(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return s, nil
}

// initAWSWatchers starts AWS resource watchers based on types provided.
func (s *Server) initAWSWatchers(matchers []services.AWSMatcher) error {
	ec2Matchers, otherMatchers := splitAWSMatchers(matchers)

	// start ec2 watchers
	var err error
	if len(ec2Matchers) > 0 {
		s.cloudWatcher, err = server.NewCloudWatcher(s.ctx, ec2Matchers, s.Clients)
		if err != nil {
			return trace.Wrap(err)
		}
		s.ec2Installer = server.NewSSMInstaller(server.SSMInstallerConfig{
			Emitter: s.Emitter,
		})
	}

	for _, matcher := range otherMatchers {
		for _, t := range matcher.Types {
			for _, region := range matcher.Regions {
				switch t {
				case constants.AWSServiceTypeEKS:
					client, err := s.Clients.GetAWSEKSClient(region)
					if err != nil {
						return trace.Wrap(err)
					}
					fetcher, err := fetchers.NewEKSFetcher(
						fetchers.EKSFetcherConfig{
							Client:       client,
							Region:       region,
							FilterLabels: matcher.Tags,
							Log:          s.Log,
						},
					)
					if err != nil {
						return trace.Wrap(err)
					}
					s.kubeFetchers = append(s.kubeFetchers, fetcher)
				}
			}
		}
	}

	return nil
}

// initAzureWatchers starts Azure resource watchers based on types provided.
func (s *Server) initAzureWatchers(ctx context.Context, matchers []services.AzureMatcher) error {
	for _, matcher := range matchers {
		subscriptions, err := s.getAzureSubscriptions(ctx, matcher.Subscriptions)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, subscription := range subscriptions {
			for _, t := range matcher.Types {
				switch t {
				case constants.AzureServiceTypeKubernetes:
					kubeClient, err := s.Clients.GetAzureKubernetesClient(subscription)
					if err != nil {
						return trace.Wrap(err)
					}
					fetcher, err := fetchers.NewAKSFetcher(fetchers.AKSFetcherConfig{
						Client:         kubeClient,
						Regions:        matcher.Regions,
						FilterLabels:   matcher.ResourceTags,
						ResourceGroups: matcher.ResourceGroups,
						Log:            s.Log,
					})
					if err != nil {
						return trace.Wrap(err)
					}
					s.kubeFetchers = append(s.kubeFetchers, fetcher)
				}
			}
		}
	}
	return nil
}

// initGCPWatchers starts GCP resource watchers based on types provided.
func (s *Server) initGCPWatchers(ctx context.Context, matchers []services.GCPMatcher) error {
	// return early if there are no matchers as GetGCPGKEClient causes
	// an error if there are no credentials present
	if len(matchers) == 0 {
		return nil
	}
	kubeClient, err := s.Clients.GetGCPGKEClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, matcher := range matchers {
		for _, projectID := range matcher.ProjectIDs {
			for _, location := range matcher.Locations {
				for _, t := range matcher.Types {
					switch t {
					case constants.GCPServiceTypeKubernetes:
						fetcher, err := fetchers.NewGKEFetcher(fetchers.GKEFetcherConfig{
							Client:       kubeClient,
							Location:     location,
							FilterLabels: matcher.Tags,
							ProjectID:    projectID,
							Log:          s.Log,
						})
						if err != nil {
							return trace.Wrap(err)
						}
						s.kubeFetchers = append(s.kubeFetchers, fetcher)
					}
				}
			}
		}
	}
	return nil
}

func (s *Server) filterExistingNodes(instances *server.EC2Instances) {
	nodes := s.nodeWatcher.GetNodes(s.ctx, func(n services.Node) bool {
		labels := n.GetAllLabels()
		_, accountOK := labels[types.AWSAccountIDLabel]
		_, instanceOK := labels[types.AWSInstanceIDLabel]
		return accountOK && instanceOK
	})

	var filtered []*ec2.Instance
outer:
	for _, inst := range instances.Instances {
		for _, node := range nodes {
			match := types.MatchLabels(node, map[string]string{
				types.AWSAccountIDLabel:  instances.AccountID,
				types.AWSInstanceIDLabel: aws.StringValue(inst.InstanceId),
			})
			if match {
				continue outer
			}
		}
		filtered = append(filtered, inst)
	}
	instances.Instances = filtered
}

func genInstancesLogStr(instances []*ec2.Instance) string {
	var logInstances strings.Builder
	for idx, inst := range instances {
		if idx == 10 || idx == (len(instances)-1) {
			logInstances.WriteString(aws.StringValue(inst.InstanceId))
			break
		}
		logInstances.WriteString(aws.StringValue(inst.InstanceId) + ", ")
	}
	if len(instances) > 10 {
		logInstances.WriteString(fmt.Sprintf("... + %d instance IDs trunacted", len(instances)-10))
	}

	return fmt.Sprintf("[%s]", logInstances.String())
}

func (s *Server) handleInstances(instances *server.EC2Instances) error {
	client, err := s.Clients.GetAWSSSMClient(instances.Region)
	if err != nil {
		return trace.Wrap(err)
	}
	s.filterExistingNodes(instances)
	if len(instances.Instances) == 0 {
		return trace.NotFound("all fetched nodes already enrolled")
	}

	s.Log.Debugf("Running Teleport installation on these instances: AccountID: %s, Instances: %s",
		instances.AccountID, genInstancesLogStr(instances.Instances))
	req := server.SSMRunRequest{
		DocumentName: instances.DocumentName,
		SSM:          client,
		Instances:    instances.Instances,
		Params:       instances.Parameters,
		Region:       instances.Region,
		AccountID:    instances.AccountID,
	}
	return trace.Wrap(s.ec2Installer.Run(s.ctx, req))
}

func (s *Server) handleEC2Discovery() {
	if err := s.nodeWatcher.WaitInitialization(); err != nil {
		s.Log.WithError(err).Error("Failed to initialize nodeWatcher.")
		return
	}

	go s.cloudWatcher.Run()
	for {
		select {
		case instances := <-s.cloudWatcher.InstancesC:
			s.Log.Debugf("EC2 instances discovered (AccountID: %s, Instances: %v), starting installation",
				instances.AccountID, genInstancesLogStr(instances.Instances))
			if err := s.handleInstances(&instances); err != nil {
				var aErr awserr.Error
				if errors.As(err, &aErr) && aErr.Code() == ssm.ErrCodeInvalidInstanceId {
					s.Log.WithError(err).Error("SSM SendCommand failed with ErrCodeInvalidInstanceId. Make sure that the instances have AmazonSSMManagedInstanceCore policy assigned. Also check that SSM agent is running and registered with the SSM endpoint on that instance and try restarting or reinstalling it in case of issues. See https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_SendCommand.html#API_SendCommand_Errors for more details.")
				} else if trace.IsNotFound(err) {
					s.Log.Debug("All discovered EC2 instances are already part of the cluster.")
				} else {
					s.Log.WithError(err).Error("Failed to enroll discovered EC2 instances.")
				}
			}
		case <-s.ctx.Done():
			s.cloudWatcher.Stop()
		}
	}
}

// Start starts the discovery service.
func (s *Server) Start() error {
	if s.cloudWatcher != nil && s.ec2Installer != nil {
		go s.handleEC2Discovery()
	}
	if len(s.kubeFetchers) > 0 {
		if err := s.startKubeWatchers(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Stop stops the discovery service.
func (s *Server) Stop() {
	s.cancelfn()
	if s.cloudWatcher != nil {
		s.cloudWatcher.Stop()
	}
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.ctx.Done()
	if err := s.ctx.Err(); err != nil && err != context.Canceled {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) getAzureSubscriptions(ctx context.Context, subs []string) ([]string, error) {
	subscriptionIds := subs
	if utils.SliceContainsStr(subs, types.Wildcard) {
		subsClient, err := s.Clients.GetAzureSubscriptionClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		subscriptionIds, err = subsClient.ListSubscriptionIDs(ctx)
		return subscriptionIds, trace.Wrap(err)
	}

	return subscriptionIds, nil
}

func (s *Server) initTeleportNodeWatcher() (err error) {
	s.nodeWatcher, err = services.NewNodeWatcher(s.ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:    teleport.ComponentDiscovery,
			Log:          s.Log,
			Client:       s.AccessPoint,
			MaxStaleness: time.Minute,
		},
	})

	return trace.Wrap(err)
}

// splitAWSMatchers splits the matchers between EC2 matchers and others.
func splitAWSMatchers(matchers []services.AWSMatcher) (ec2 []services.AWSMatcher, other []services.AWSMatcher) {
	for _, matcher := range matchers {
		if utils.SliceContainsStr(matcher.Types, constants.AWSServiceTypeEC2) {
			ec2 = append(ec2,
				copyAWSMatcherWithNewTypes(matcher, []string{constants.AWSServiceTypeEC2}),
			)
		}

		otherTypes := excludeFromSlice(matcher.Types, constants.AWSServiceTypeEC2)
		if len(otherTypes) > 0 {
			other = append(other, copyAWSMatcherWithNewTypes(matcher, otherTypes))
		}
	}
	return
}

// excludeFromSlice excludes entry from the slice.
func excludeFromSlice[T comparable](slice []T, entry T) []T {
	newSlice := make([]T, 0, len(slice))
	for _, val := range slice {
		if val != entry {
			newSlice = append(newSlice, val)
		}
	}
	return newSlice
}

// copyAWSMatcherWithNewTypes copies an AWS Matcher and replaces the types with newTypes
func copyAWSMatcherWithNewTypes(matcher services.AWSMatcher, newTypes []string) services.AWSMatcher {
	newMatcher := matcher
	newMatcher.Types = newTypes
	return newMatcher
}
