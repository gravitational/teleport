/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"context"
	"io"
	"sync"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"google.golang.org/grpc"
)

// CloudClients provides interface for obtaining cloud provider clients.
type CloudClients interface {
	// GetAWSSession returns AWS session for the specified region.
	GetAWSSession(region string) (*awssession.Session, error)
	// GetAWSRDSClient returns AWS RDS client for the specified region.
	GetAWSRDSClient(region string) (rdsiface.RDSAPI, error)
	// GetAWSIAMClient returns AWS IAM client for the specified region.
	GetAWSIAMClient(region string) (iamiface.IAMAPI, error)
	// GetAWSSTSClient returns AWS STS client for the specified region.
	GetAWSSTSClient(region string) (stsiface.STSAPI, error)
	// GetGCPIAMClient returns GCP IAM client.
	GetGCPIAMClient(context.Context) (*gcpcredentials.IamCredentialsClient, error)
	// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
	GetGCPSQLAdminClient(context.Context) (*sqladmin.Service, error)
	// Closer closes all initialized clients.
	io.Closer
}

// NewCloudClients returns a new instance of cloud clients retriever.
func NewCloudClients() CloudClients {
	return &cloudClients{
		awsSessions: make(map[string]*awssession.Session),
	}
}

type cloudClients struct {
	// awsSessions is a map of cached AWS sessions per region.
	awsSessions map[string]*awssession.Session
	// gcpIAM is the cached GCP IAM client.
	gcpIAM *gcpcredentials.IamCredentialsClient
	// gcpSQLAdmin is the cached GCP Cloud SQL Admin client.
	gcpSQLAdmin *sqladmin.Service
	// mtx is used for locking.
	mtx sync.RWMutex
}

// GetAWSSession returns AWS session for the specified region.
func (c *cloudClients) GetAWSSession(region string) (*awssession.Session, error) {
	c.mtx.RLock()
	if session, ok := c.awsSessions[region]; ok {
		c.mtx.RUnlock()
		return session, nil
	}
	c.mtx.RUnlock()
	return c.initAWSSession(region)
}

// GetAWSRDSClient returns AWS RDS client for the specified region.
func (c *cloudClients) GetAWSRDSClient(region string) (rdsiface.RDSAPI, error) {
	session, err := c.GetAWSSession(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rds.New(session), nil
}

// GetAWSIAMClient returns AWS IAM client for the specified region.
func (c *cloudClients) GetAWSIAMClient(region string) (iamiface.IAMAPI, error) {
	session, err := c.GetAWSSession(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return iam.New(session), nil
}

// GetAWSSTSClient returns AWS STS client for the specified region.
func (c *cloudClients) GetAWSSTSClient(region string) (stsiface.STSAPI, error) {
	session, err := c.GetAWSSession(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sts.New(session), nil
}

// GetGCPIAMClient returns GCP IAM client.
func (c *cloudClients) GetGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.RLock()
	if c.gcpIAM != nil {
		c.mtx.RUnlock()
		return c.gcpIAM, nil
	}
	c.mtx.RUnlock()
	return c.initGCPIAMClient(ctx)
}

// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *cloudClients) GetGCPSQLAdminClient(ctx context.Context) (*sqladmin.Service, error) {
	c.mtx.RLock()
	if c.gcpSQLAdmin != nil {
		c.mtx.RUnlock()
		return c.gcpSQLAdmin, nil
	}
	c.mtx.RUnlock()
	return c.initGCPSQLAdminClient(ctx)
}

// Closes closes all initialized clients.
func (c *cloudClients) Close() (err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpIAM != nil {
		err = c.gcpIAM.Close()
		c.gcpIAM = nil
	}
	return trace.Wrap(err)
}

func (c *cloudClients) initAWSSession(region string) (*awssession.Session, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if session, ok := c.awsSessions[region]; ok { // If some other thead already got here first.
		return session, nil
	}
	logrus.Debugf("Initializing AWS session for region %v.", region)
	session, err := awssession.NewSessionWithOptions(awssession.Options{
		SharedConfigState: awssession.SharedConfigEnable,
		Config: aws.Config{
			Region: aws.String(region),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.awsSessions[region] = session
	return session, nil
}

func (c *cloudClients) initGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpIAM != nil { // If some other thread already got here first.
		return c.gcpIAM, nil
	}
	logrus.Debug("Initializing GCP IAM client.")
	gcpIAM, err := gcpcredentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gcpIAM = gcpIAM
	return gcpIAM, nil
}

func (c *cloudClients) initGCPSQLAdminClient(ctx context.Context) (*sqladmin.Service, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.gcpSQLAdmin != nil { // If some other thread already got here first.
		return c.gcpSQLAdmin, nil
	}
	logrus.Debug("Initializing GCP Cloud SQL Admin client.")
	gcpSQLAdmin, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gcpSQLAdmin = gcpSQLAdmin
	return gcpSQLAdmin, nil
}

// TestCloudClients are used in tests.
type TestCloudClients struct {
	RDS rdsiface.RDSAPI
	IAM iamiface.IAMAPI
	STS stsiface.STSAPI
}

// GetAWSSession returns AWS session for the specified region.
func (c *TestCloudClients) GetAWSSession(region string) (*awssession.Session, error) {
	return nil, trace.NotImplemented("not implemented")
}

// GetAWSRDSClient returns AWS RDS client for the specified region.
func (c *TestCloudClients) GetAWSRDSClient(region string) (rdsiface.RDSAPI, error) {
	return c.RDS, nil
}

// GetAWSIAMClient returns AWS IAM client for the specified region.
func (c *TestCloudClients) GetAWSIAMClient(region string) (iamiface.IAMAPI, error) {
	return c.IAM, nil
}

// GetAWSSTSClient returns AWS STS client for the specified region.
func (c *TestCloudClients) GetAWSSTSClient(region string) (stsiface.STSAPI, error) {
	return c.STS, nil
}

// GetGCPIAMClient returns GCP IAM client.
func (c *TestCloudClients) GetGCPIAMClient(ctx context.Context) (*gcpcredentials.IamCredentialsClient, error) {
	return gcpcredentials.NewIamCredentialsClient(ctx,
		option.WithGRPCDialOption(grpc.WithInsecure()), // Insecure must be set for unauth client.
		option.WithoutAuthentication())
}

// GetGCPSQLAdminClient returns GCP Cloud SQL Admin client.
func (c *TestCloudClients) GetGCPSQLAdminClient(ctx context.Context) (*sqladmin.Service, error) {
	return sqladmin.NewService(ctx,
		option.WithGRPCDialOption(grpc.WithInsecure()), // Insecure must be set for unauth client.
		option.WithoutAuthentication())
}

// Closer closes all initialized clients.
func (c *TestCloudClients) Close() error {
	return nil
}
