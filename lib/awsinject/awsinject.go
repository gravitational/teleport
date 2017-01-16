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

// package awsinject injects AWS metadata as labels
package awsinject

import (
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gravitational/trace"
)

// Tags injects tags as server labels, it is run on
// Auth Server side, as it can require special IAM role or API keys
// to query instance tags that are not available as instance metadata
type Tags struct {
	sync.Mutex
	sess     *session.Session
	services map[string]*ec2.EC2
}

// NewTags returns new tags injector
func NewTags() (*Tags, error) {
	// create an AWS session using default SDK behavior, i.e. it will interpret
	// the environment and ~/.aws directory just like an AWS CLI tool would:
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Tags{
		sess:     sess,
		services: make(map[string]*ec2.EC2),
	}, nil
}

func (t *Tags) getService(region string) *ec2.EC2 {
	t.Lock()
	defer t.Unlock()
	service, ok := t.services[region]
	if ok {
		return service
	}
	service = ec2.New(t.sess, aws.NewConfig().WithRegion(region))
	t.services[region] = service
	return service
}

// Inject injects AWS tags for this server
func (t *Tags) Inject(s services.Server) error {
	instanceID, ok := s.GetLabel(teleport.AWSInstanceIDLabel)
	if !ok || instanceID == "" {
		log.Debugf("%v server has no instance ID specified", s.GetAddr())
		return nil
	}
	instanceRegion, ok := s.GetLabel(teleport.AWSInstanceRegionLabel)
	if !ok || instanceRegion == "" {
		log.Debugf("%v server has no instance regsion specified", s.GetAddr())
		return nil
	}
	service := t.getService(instanceRegion)
	// delete label to avoid extra noise
	s.DeleteLabel(teleport.AWSInstanceIDLabel)
	s.DeleteLabel(teleport.AWSInstanceRegionLabel)
	params := &ec2.DescribeTagsInput{
		DryRun: aws.Bool(false),
		Filters: []*ec2.Filter{
			{
				Name: aws.String("resource-id"),
				Values: []*string{
					aws.String(instanceID),
				},
			},
		},
		MaxResults: aws.Int64(20),
	}
	resp, err := service.DescribeTags(params)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, tag := range resp.Tags {
		if tag != nil && tag.Key != nil && tag.Value != nil {
			s.SetLabel(*tag.Key, *tag.Value)
		}
	}
	return nil
}

// Metadata injects instance ID as Teleport label,
// Every Teleport SSH node runs it, it helps to communicate instance
// ID to the auth server, it does not require any IAM role or API keys
// to be present on Node's instance
type Metadata struct {
	session *session.Session
}

// NewMetadata returns metadata injector
func NewMetadata() (*Metadata, error) {
	session := session.New()
	return &Metadata{session: session}, nil
}

// Inject injects instance id as teleport label
func (m *Metadata) Inject(s services.Server) error {
	metadata := ec2metadata.New(m.session)
	identity, err := metadata.GetInstanceIdentityDocument()
	if err != nil {
		return trace.Wrap(err, "failed to fetch instance metadata")
	}
	s.SetLabel(teleport.AWSInstanceIDLabel, identity.InstanceID)
	s.SetLabel(teleport.AWSInstanceRegionLabel, identity.Region)
	return nil
}
