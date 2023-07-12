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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
	"github.com/gravitational/teleport/lib/srv/server"
)

var errNoInstances = errors.New("all fetched nodes already enrolled")

// Config provides configuration for the discovery server.
type Config struct {
	// Clients is an interface for retrieving cloud clients.
	Clients cloud.Clients
	// AWSMatchers is a list of AWS EC2 matchers.
	AWSMatchers []types.AWSMatcher
	// AzureMatchers is a list of Azure matchers to discover resources.
	AzureMatchers []types.AzureMatcher
	// GCPMatchers is a list of GCP matchers to discover resources.
	GCPMatchers []types.GCPMatcher
	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter
	// AccessPoint is a discovery access point
	AccessPoint auth.DiscoveryAccessPoint
	// Log is the logger.
	Log logrus.FieldLogger
	// onDatabaseReconcile is called after each database resource reconciliation.
	onDatabaseReconcile func()
	// DiscoveryGroup is the name of the discovery group that the current
	// discovery service is a part of.
	// It is used to filter out discovered resources that belong to another
	// discovery services. When running in high availability mode and the agents
	// have access to the same cloud resources, this field value must be the same
	// for all discovery services. If different agents are used to discover different
	// sets of cloud resources, this field must be different for each set of agents.
	DiscoveryGroup string
	// ClusterName is the name of the Teleport cluster.
	ClusterName string
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Clients == nil {
		cloudClients, err := cloud.NewClients()
		if err != nil {
			return trace.Wrap(err)
		}
		c.Clients = cloudClients
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
	c.AzureMatchers = services.SimplifyAzureMatchers(c.AzureMatchers)
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

	// ec2Watcher periodically retrieves EC2 instances.
	ec2Watcher *server.Watcher
	// ec2Installer is used to start the installation process on discovered EC2 nodes
	ec2Installer *server.SSMInstaller
	// azureWatcher periodically retrieves Azure virtual machines.
	azureWatcher *server.Watcher
	// azureInstaller is used to start the installation process on discovered Azure
	// virtual machines.
	azureInstaller *server.AzureInstaller
	// kubeFetchers holds all kubernetes fetchers for Azure and other clouds.
	kubeFetchers []common.Fetcher
	// databaseFetchers holds all database fetchers.
	databaseFetchers []common.Fetcher
	// caRotationCh receives nodes that need to have their CAs rotated.
	caRotationCh chan []types.Server
	// reconciler periodically reconciles the labels of discovered instances
	// with the auth server.
	reconciler *labelReconciler
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

	if s.ec2Watcher != nil || s.azureWatcher != nil {
		if err := s.initTeleportNodeWatcher(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return s, nil
}

// initAWSWatchers starts AWS resource watchers based on types provided.
func (s *Server) initAWSWatchers(matchers []types.AWSMatcher) error {
	ec2Matchers, otherMatchers := splitAWSMatchers(matchers, func(matcherType string) bool {
		return matcherType == services.AWSMatcherEC2
	})

	// start ec2 watchers
	var err error
	if len(ec2Matchers) > 0 {
		s.caRotationCh = make(chan []types.Server)
		s.ec2Watcher, err = server.NewEC2Watcher(s.ctx, ec2Matchers, s.Clients, s.caRotationCh)
		if err != nil {
			return trace.Wrap(err)
		}

		s.ec2Installer = server.NewSSMInstaller(server.SSMInstallerConfig{
			Emitter: s.Emitter,
		})
		lr, err := newLabelReconciler(&labelReconcilerConfig{
			log:         s.Log,
			accessPoint: s.AccessPoint,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		s.reconciler = lr
	}

	// Add database fetchers.
	databaseMatchers, otherMatchers := splitAWSMatchers(otherMatchers, db.IsAWSMatcherType)
	if len(databaseMatchers) > 0 {
		databaseFetchers, err := db.MakeAWSFetchers(s.ctx, s.Clients, databaseMatchers)
		if err != nil {
			return trace.Wrap(err)
		}
		s.databaseFetchers = append(s.databaseFetchers, databaseFetchers...)
	}

	// Add kube fetchers.
	for _, matcher := range otherMatchers {
		matcherAssumeRole := &types.AssumeRole{}
		if matcher.AssumeRole != nil {
			matcherAssumeRole = matcher.AssumeRole
		}

		for _, t := range matcher.Types {
			for _, region := range matcher.Regions {
				switch t {
				case services.AWSMatcherEKS:
					client, err := s.Clients.GetAWSEKSClient(
						s.ctx,
						region,
						cloud.WithAssumeRole(
							matcherAssumeRole.RoleARN,
							matcherAssumeRole.ExternalID,
						),
					)
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
func (s *Server) initAzureWatchers(ctx context.Context, matchers []types.AzureMatcher) error {
	vmMatchers, otherMatchers := splitAzureMatchers(matchers, func(matcherType string) bool {
		return matcherType == services.AzureMatcherVM
	})

	// VM watcher.
	if len(vmMatchers) > 0 {
		var err error
		s.azureWatcher, err = server.NewAzureWatcher(s.ctx, vmMatchers, s.Clients)
		if err != nil {
			return trace.Wrap(err)
		}
		s.azureInstaller = &server.AzureInstaller{
			Emitter:     s.Emitter,
			AccessPoint: s.AccessPoint,
		}
	}

	// Add database fetchers.
	databaseMatchers, otherMatchers := splitAzureMatchers(otherMatchers, db.IsAzureMatcherType)
	if len(databaseMatchers) > 0 {
		databaseFetchers, err := db.MakeAzureFetchers(s.Clients, databaseMatchers)
		if err != nil {
			return trace.Wrap(err)
		}
		s.databaseFetchers = append(s.databaseFetchers, databaseFetchers...)
	}

	// Add kube fetchers.
	for _, matcher := range otherMatchers {
		subscriptions, err := s.getAzureSubscriptions(ctx, matcher.Subscriptions)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, subscription := range subscriptions {
			for _, t := range matcher.Types {
				switch t {
				case services.AzureMatcherKubernetes:
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
func (s *Server) initGCPWatchers(ctx context.Context, matchers []types.GCPMatcher) error {
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
					case services.GCPMatcherKubernetes:
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

func (s *Server) filterExistingEC2Nodes(instances *server.EC2Instances) {
	nodes := s.nodeWatcher.GetNodes(s.ctx, func(n services.Node) bool {
		labels := n.GetAllLabels()
		_, accountOK := labels[types.AWSAccountIDLabel]
		_, instanceOK := labels[types.AWSInstanceIDLabel]
		return accountOK && instanceOK
	})

	var filtered []server.EC2Instance
outer:
	for _, inst := range instances.Instances {
		for _, node := range nodes {
			match := types.MatchLabels(node, map[string]string{
				types.AWSAccountIDLabel:  instances.AccountID,
				types.AWSInstanceIDLabel: inst.InstanceID,
			})
			if match {
				continue outer
			}
		}
		filtered = append(filtered, inst)
	}
	instances.Instances = filtered
}

func genEC2InstancesLogStr(instances []server.EC2Instance) string {
	return genInstancesLogStr(instances, func(i server.EC2Instance) string {
		return i.InstanceID
	})
}

func genAzureInstancesLogStr(instances []*armcompute.VirtualMachine) string {
	return genInstancesLogStr(instances, func(i *armcompute.VirtualMachine) string {
		return aws.StringValue(i.Name)
	})
}

func genInstancesLogStr[T any](instances []T, getID func(T) string) string {
	var logInstances strings.Builder
	for idx, inst := range instances {
		if idx == 10 || idx == (len(instances)-1) {
			logInstances.WriteString(getID(inst))
			break
		}
		logInstances.WriteString(getID(inst) + ", ")
	}
	if len(instances) > 10 {
		logInstances.WriteString(fmt.Sprintf("... + %d instance IDs truncated", len(instances)-10))
	}

	return fmt.Sprintf("[%s]", logInstances.String())
}

func (s *Server) handleEC2Instances(instances *server.EC2Instances) error {
	// TODO(gavin): support assume_role_arn for ec2.
	ec2Client, err := s.Clients.GetAWSSSMClient(s.ctx, instances.Region)
	if err != nil {
		return trace.Wrap(err)
	}

	serverInfos, err := instances.ServerInfos()
	if err != nil {
		return trace.Wrap(err)
	}
	s.reconciler.queueServerInfos(serverInfos)

	// instances.Rotation is true whenever the instances received need
	// to be rotated, we don't want to filter out existing OpenSSH nodes as
	// they all need to have the command run on them
	if !instances.Rotation {
		s.filterExistingEC2Nodes(instances)
	}
	if len(instances.Instances) == 0 {
		return trace.NotFound("all fetched nodes already enrolled")
	}

	s.Log.Debugf("Running Teleport installation on these instances: AccountID: %s, Instances: %s",
		instances.AccountID, genEC2InstancesLogStr(instances.Instances))

	req := server.SSMRunRequest{
		DocumentName: instances.DocumentName,
		SSM:          ec2Client,
		Instances:    instances.Instances,
		Params:       instances.Parameters,
		Region:       instances.Region,
		AccountID:    instances.AccountID,
	}
	return trace.Wrap(s.ec2Installer.Run(s.ctx, req))
}

func (s *Server) logHandleInstancesErr(err error) {
	var aErr awserr.Error
	if errors.As(err, &aErr) && aErr.Code() == ssm.ErrCodeInvalidInstanceId {
		s.Log.WithError(err).Error("SSM SendCommand failed with ErrCodeInvalidInstanceId. Make sure that the instances have AmazonSSMManagedInstanceCore policy assigned. Also check that SSM agent is running and registered with the SSM endpoint on that instance and try restarting or reinstalling it in case of issues. See https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_SendCommand.html#API_SendCommand_Errors for more details.")
	} else if trace.IsNotFound(err) {
		s.Log.Debug("All discovered EC2 instances are already part of the cluster.")
	} else {
		s.Log.WithError(err).Error("Failed to enroll discovered EC2 instances.")
	}
}

func (s *Server) watchCARotation(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			nodes, err := s.findUnrotatedEC2Nodes(ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					s.Log.Debug("No OpenSSH nodes require CA rotation")
					continue
				}
				s.Log.Errorf("Error finding OpenSSH nodes requiring CA rotation: %s", err)
				continue
			}
			s.Log.Debugf("Found %d nodes requiring rotation", len(nodes))
			s.caRotationCh <- nodes
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) getMostRecentRotationForCAs(ctx context.Context, caTypes ...types.CertAuthType) (time.Time, error) {
	var mostRecentUpdate time.Time
	for _, caType := range caTypes {
		ca, err := s.AccessPoint.GetCertAuthority(ctx, types.CertAuthID{
			Type:       caType,
			DomainName: s.ClusterName,
		}, false)
		if err != nil {
			return time.Time{}, trace.Wrap(err)
		}
		caRot := ca.GetRotation()
		if caRot.State == types.RotationStateInProgress && caRot.Started.After(mostRecentUpdate) {
			mostRecentUpdate = caRot.Started
		}

		if caRot.LastRotated.After(mostRecentUpdate) {
			mostRecentUpdate = caRot.LastRotated
		}
	}
	return mostRecentUpdate, nil
}

func (s *Server) findUnrotatedEC2Nodes(ctx context.Context) ([]types.Server, error) {
	mostRecentCertRotation, err := s.getMostRecentRotationForCAs(ctx, types.OpenSSHCA, types.HostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	found := s.nodeWatcher.GetNodes(ctx, func(n services.Node) bool {
		if n.GetSubKind() != types.SubKindOpenSSHNode {
			return false
		}
		if _, ok := n.GetLabel(types.AWSAccountIDLabel); !ok {
			return false
		}
		if _, ok := n.GetLabel(types.AWSInstanceIDLabel); !ok {
			return false
		}

		return mostRecentCertRotation.After(n.GetRotation().LastRotated)
	})

	if len(found) == 0 {
		return nil, trace.NotFound("no unrotated nodes found")
	}
	return found, nil
}

func (s *Server) handleEC2Discovery() {
	if err := s.nodeWatcher.WaitInitialization(); err != nil {
		s.Log.WithError(err).Error("Failed to initialize nodeWatcher.")
		return
	}

	go s.ec2Watcher.Run()
	go s.watchCARotation(s.ctx)

	for {
		select {
		case instances := <-s.ec2Watcher.InstancesC:
			ec2Instances := instances.EC2Instances
			s.Log.Debugf("EC2 instances discovered (AccountID: %s, Instances: %v), starting installation",
				instances.AccountID, genEC2InstancesLogStr(ec2Instances.Instances))

			if err := s.handleEC2Instances(ec2Instances); err != nil {
				s.logHandleInstancesErr(err)
			}
		case <-s.ctx.Done():
			s.ec2Watcher.Stop()
			return
		}
	}
}

func (s *Server) filterExistingAzureNodes(instances *server.AzureInstances) {
	nodes := s.nodeWatcher.GetNodes(s.ctx, func(n services.Node) bool {
		labels := n.GetAllLabels()
		_, subscriptionOK := labels[types.SubscriptionIDLabel]
		_, vmOK := labels[types.VMIDLabel]
		return subscriptionOK && vmOK
	})
	var filtered []*armcompute.VirtualMachine
outer:
	for _, inst := range instances.Instances {
		for _, node := range nodes {
			var vmID string
			if inst.Properties != nil {
				vmID = aws.StringValue(inst.Properties.VMID)
			}
			match := types.MatchLabels(node, map[string]string{
				types.SubscriptionIDLabel: instances.SubscriptionID,
				types.VMIDLabel:           vmID,
			})
			if match {
				continue outer
			}
		}
		filtered = append(filtered, inst)
	}
	instances.Instances = filtered
}

func (s *Server) handleAzureInstances(instances *server.AzureInstances) error {
	client, err := s.Clients.GetAzureRunCommandClient(instances.SubscriptionID)
	if err != nil {
		return trace.Wrap(err)
	}
	s.filterExistingAzureNodes(instances)
	if len(instances.Instances) == 0 {
		return trace.Wrap(errNoInstances)
	}

	s.Log.Debugf("Running Teleport installation on these virtual machines: SubscriptionID: %s, VMs: %s",
		instances.SubscriptionID, genAzureInstancesLogStr(instances.Instances),
	)
	req := server.AzureRunRequest{
		Client:          client,
		Instances:       instances.Instances,
		Region:          instances.Region,
		ResourceGroup:   instances.ResourceGroup,
		Params:          instances.Parameters,
		ScriptName:      instances.ScriptName,
		PublicProxyAddr: instances.PublicProxyAddr,
	}
	return trace.Wrap(s.azureInstaller.Run(s.ctx, req))
}

func (s *Server) handleAzureDiscovery() {
	if err := s.nodeWatcher.WaitInitialization(); err != nil {
		s.Log.WithError(err).Error("Failed to initialize nodeWatcher.")
		return
	}

	go s.azureWatcher.Run()
	for {
		select {
		case instances := <-s.azureWatcher.InstancesC:
			azureInstances := instances.AzureInstances
			s.Log.Debugf("Azure instances discovered (SubscriptionID: %s, Instances: %v), starting installation",
				instances.SubscriptionID, genAzureInstancesLogStr(azureInstances.Instances),
			)
			if err := s.handleAzureInstances(azureInstances); err != nil {
				if errors.Is(err, errNoInstances) {
					s.Log.Debug("All discovered Azure VMs are already part of the cluster.")
				} else {
					s.Log.WithError(err).Error("Failed to enroll discovered Azure VMs.")
				}
			}
		case <-s.ctx.Done():
			s.azureWatcher.Stop()
			return
		}
	}
}

// Start starts the discovery service.
func (s *Server) Start() error {
	if s.ec2Watcher != nil {
		go s.handleEC2Discovery()
		go s.reconciler.run(s.ctx)
	}
	if s.azureWatcher != nil {
		go s.handleAzureDiscovery()
	}
	if len(s.kubeFetchers) > 0 {
		if err := s.startKubeWatchers(); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := s.startDatabaseWatchers(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Stop stops the discovery service.
func (s *Server) Stop() {
	s.cancelfn()
	if s.ec2Watcher != nil {
		s.ec2Watcher.Stop()
	}
	if s.azureWatcher != nil {
		s.azureWatcher.Stop()
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
	if slices.Contains(subs, types.Wildcard) {
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

// splitSlice splits a slice into two, by putting all elements that satisfy the
// provided check function in the first slice, while putting all other elements
// in the second slice.
func splitSlice(ss []string, check func(string) bool) (split, other []string) {
	for _, e := range ss {
		if check(e) {
			split = append(split, e)
		} else {
			other = append(other, e)
		}
	}
	return
}

// splitAWSMatchers splits the AWS matchers by checking the matcher types.
func splitAWSMatchers(matchers []types.AWSMatcher, matcherTypeCheck func(string) bool) (split, other []types.AWSMatcher) {
	for _, matcher := range matchers {
		splitTypes, otherTypes := splitSlice(matcher.Types, matcherTypeCheck)

		if len(splitTypes) > 0 {
			split = append(split, copyAWSMatcherWithNewTypes(matcher, splitTypes))
		}
		if len(otherTypes) > 0 {
			other = append(other, copyAWSMatcherWithNewTypes(matcher, otherTypes))
		}
	}
	return
}

// splitAzureMatchers splits the Azure matchers by checking the matcher types.
func splitAzureMatchers(matchers []types.AzureMatcher, matcherTypeCheck func(string) bool) (split, other []types.AzureMatcher) {
	for _, matcher := range matchers {
		splitTypes, otherTypes := splitSlice(matcher.Types, matcherTypeCheck)

		if len(splitTypes) > 0 {
			split = append(split, copyAzureMatcherWithNewTypes(matcher, splitTypes))
		}
		if len(otherTypes) > 0 {
			other = append(other, copyAzureMatcherWithNewTypes(matcher, otherTypes))
		}
	}
	return
}

// copyAWSMatcherWithNewTypes copies an AWS Matcher and replaces the types with newTypes
func copyAWSMatcherWithNewTypes(matcher types.AWSMatcher, newTypes []string) types.AWSMatcher {
	newMatcher := matcher
	newMatcher.Types = newTypes
	return newMatcher
}

// copyAzureMatcherWithNewTypes copies an Azure Matcher and replaces the types with newTypes.
func copyAzureMatcherWithNewTypes(matcher types.AzureMatcher, newTypes []string) types.AzureMatcher {
	newMatcher := matcher
	newMatcher.Types = newTypes
	return newMatcher
}
