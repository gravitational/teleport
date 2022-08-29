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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/server"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Clients     cloud.Clients
	Matchers    []services.AWSMatcher
	NodeWatcher *services.NodeWatcher
	Emitter     events.StreamEmitter
	AccessPoint auth.DiscoveryAccessPoint
}

// Server is a discovery server, It
type Server struct {
	*Config
	events.StreamEmitter

	ctx context.Context

	// log is used for logging.
	log *logrus.Entry

	// cloudWatcher periodically retrieves cloud resources, currently
	// only EC2
	cloudWatcher *server.Watcher
	// ec2Installer is used to start the installation process on discovered EC2 nodes
	ec2Installer *server.SSMInstaller
}

func New(
	ctx context.Context,
	cfg *Config) (*Server, error) {
	s := &Server{
		Config:        cfg,
		StreamEmitter: cfg.Emitter,
		log: logrus.WithFields(logrus.Fields{
			trace.Component:       teleport.ComponentDiscovery,
			trace.ComponentFields: logrus.Fields{},
		}),
		ctx: ctx,
	}
	var err error
	if len(s.Matchers) == 0 {
		return nil, trace.NotFound("no matchers present")
	}

	s.cloudWatcher, err = server.NewCloudWatcher(s.ctx, s.Matchers, s.Clients)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.ec2Installer = server.NewSSMInstaller(server.SSMInstallerConfig{
		Emitter: s,
	})

	return s, nil
}

func (s *Server) filterExistingNodes(instances *server.EC2Instances) {
	nodes := s.NodeWatcher.GetNodes(func(n services.Node) bool {
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

func (s *Server) handleInstances(instances *server.EC2Instances) error {
	client, err := s.Clients.GetAWSSSMClient(instances.Region)
	if err != nil {
		return trace.Wrap(err)
	}
	s.filterExistingNodes(instances)
	if len(instances.Instances) == 0 {
		s.log.Debugf("All fetched nodes already enrolled.")
		return nil
	}

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
	go s.cloudWatcher.Run()
	for {
		select {
		case instances := <-s.cloudWatcher.InstancesC:
			s.log.Debugln("EC2 instances discovered, starting installation")
			if err := s.handleInstances(&instances); err != nil {
				if trace.IsNotFound(err) {
					s.log.Debug("All discovered EC2 instances are already part of the cluster.")
				} else {
					s.log.WithError(err).Error("Failed to enroll discovered EC2 instances.")
				}
			}
		case <-s.ctx.Done():
			s.cloudWatcher.Stop()
		}
	}
}

func (s *Server) Start() error {
	go s.handleEC2Discovery()
	return nil
}

func (s *Server) Stop() {
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
