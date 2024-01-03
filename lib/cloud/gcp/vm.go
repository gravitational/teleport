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

package gcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"

	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/native"
)

// sshUser is the user to log in as on GCP VMs.
const sshUser = "teleport"

// sshDefaultTimeout is the default timeout for dialing an instance.
const sshDefaultTimeout = 10 * time.Second

// convertAPIError converts an error from the GCP API into a trace error.
func convertAPIError(err error) error {
	var googleError *googleapi.Error
	if errors.As(err, &googleError) {
		return trace.ReadError(googleError.Code, []byte(googleError.Message))
	}
	return err
}

// InstanceClient is a client to interact with GCP VMs.
type InstancesClient interface {
	// ListInstances lists the GCP VMs that belong to the given project and
	// zone.
	// zone supports wildcard "*".
	ListInstances(ctx context.Context, projectID, zone string) ([]*Instance, error)
	// StreamInstances streams the GCP VMs that belong to the given project and
	// zone.
	// zone supports wildcard "*".
	StreamInstances(ctx context.Context, projectID, zone string) stream.Stream[*Instance]
	// GetInstance gets a GCP VM.
	GetInstance(ctx context.Context, req *InstanceRequest) (*Instance, error)
	// AddSSHKey adds an SSH key to a GCP VM's metadata.
	AddSSHKey(ctx context.Context, req *SSHKeyRequest) error
	// RemoveSSHKey removes an SSH key from a GCP VM's metadata.
	RemoveSSHKey(ctx context.Context, req *SSHKeyRequest) error
}

// InstancesClientConfig is the client configuration for InstancesClient.
type InstancesClientConfig struct {
	// InstanceClient is the underlying GCP client for the instances service.
	InstanceClient *compute.InstancesClient
}

// CheckAndSetDefaults checks and sets defaults for InstancesClientConfig.
func (c *InstancesClientConfig) CheckAndSetDefaults(ctx context.Context) (err error) {
	if c.InstanceClient == nil {
		if c.InstanceClient, err = compute.NewInstancesRESTClient(ctx); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Instance represents a GCP VM.
type Instance struct {
	// Name is the instance's name.
	Name string
	// Zone is the instance's zone.
	Zone string
	// ProjectID is the ID of the project the VM is in.
	ProjectID string
	// ServiceAccount is the email address of the VM's service account, if any.
	ServiceAccount string
	// Labels is the instance's labels.
	Labels map[string]string

	internalIPAddress string
	externalIPAddress string
	hostKeys          []ssh.PublicKey
	metadata          *computepb.Metadata
}

// InstanceRequest formats an instance request based on an instance.
func (i *Instance) InstanceRequest() InstanceRequest {
	return InstanceRequest{
		ProjectID: i.ProjectID,
		Zone:      i.Zone,
		Name:      i.Name,
	}
}

// NewInstancesClient creates a new InstancesClient.
func NewInstancesClient(ctx context.Context) (InstancesClient, error) {
	var cfg InstancesClientConfig
	client, err := NewInstancesClientWithConfig(ctx, cfg)
	return client, trace.Wrap(err)
}

// NewInstancesClientWithConfig creates a new InstancesClient with custom
// config.
func NewInstancesClientWithConfig(ctx context.Context, cfg InstancesClientConfig) (InstancesClient, error) {
	if err := cfg.CheckAndSetDefaults(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &instancesClient{InstancesClientConfig: cfg}, nil
}

// instancesClient implements the InstancesClient interface by wrapping
// compute.InstancesClient.
type instancesClient struct {
	InstancesClientConfig
}

// externalNATKeys is the set of keys in a GCP instance's metadata that could
// hold its external IP
var externalNATKeys = []string{
	"external-nat",
	"External NAT",
}

func isExternalNAT(s string) bool {
	return slices.Contains(externalNATKeys, s)
}

func toInstance(origInstance *computepb.Instance, projectID string) *Instance {
	zoneParts := strings.Split(origInstance.GetZone(), "/")
	zone := zoneParts[len(zoneParts)-1]
	var internalIP string
	var externalIP string
	for _, netInterface := range origInstance.GetNetworkInterfaces() {
		if internalIP == "" {
			internalIP = netInterface.GetNetworkIP()
		}
		if externalIP == "" {
			for _, accessConfig := range netInterface.GetAccessConfigs() {
				if isExternalNAT(accessConfig.GetName()) {
					externalIP = accessConfig.GetNatIP()
					break
				}
			}
		}
	}
	inst := &Instance{
		Name:              origInstance.GetName(),
		ProjectID:         projectID,
		Zone:              zone,
		Labels:            origInstance.GetLabels(),
		internalIPAddress: internalIP,
		externalIPAddress: externalIP,
		metadata:          origInstance.GetMetadata(),
	}
	// GCP VMs can have at most one service account.
	if len(origInstance.ServiceAccounts) > 0 {
		inst.ServiceAccount = origInstance.ServiceAccounts[0].GetEmail()
	}
	return inst
}

func toInstances(origInstances []*computepb.Instance, projectID string) []*Instance {
	instances := make([]*Instance, 0, len(origInstances))
	for _, inst := range origInstances {
		instances = append(instances, toInstance(inst, projectID))
	}
	return instances
}

// ListInstances lists the GCP VMs that belong to the given project and
// zone.
// zone supports wildcard "*".
func (clt *instancesClient) ListInstances(ctx context.Context, projectID, zone string) ([]*Instance, error) {
	instances, err := stream.Collect(clt.StreamInstances(ctx, projectID, zone))
	return instances, trace.Wrap(err)
}

func (clt *instancesClient) StreamInstances(ctx context.Context, projectID, zone string) stream.Stream[*Instance] {
	if len(projectID) == 0 {
		return stream.Fail[*Instance](trace.BadParameter("projectID must be set"))
	}
	if len(zone) == 0 {
		return stream.Fail[*Instance](trace.BadParameter("location must be set"))
	}

	var getInstances func() ([]*Instance, error)

	if zone == types.Wildcard {
		it := clt.InstanceClient.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{
			Project: projectID,
		})
		getInstances = func() ([]*Instance, error) {
			resp, err := it.Next()
			if resp.Value == nil {
				return nil, trace.Wrap(err)
			}
			return toInstances(resp.Value.GetInstances(), projectID), trace.Wrap(err)
		}
	} else {
		it := clt.InstanceClient.List(ctx, &computepb.ListInstancesRequest{
			Project: projectID,
			Zone:    zone,
		})
		getInstances = func() ([]*Instance, error) {
			resp, err := it.Next()
			if resp == nil {
				return nil, trace.Wrap(err)
			}
			return []*Instance{toInstance(resp, projectID)}, trace.Wrap(err)
		}
	}

	return stream.PageFunc(func() ([]*Instance, error) {
		instances, err := getInstances()
		if errors.Is(err, iterator.Done) {
			return instances, io.EOF
		}
		return instances, trace.Wrap(err)
	})
}

// InstanceRequest contains parameters for making a request to a specific instance.
type InstanceRequest struct {
	// ProjectID is the ID of the VM's project.
	ProjectID string
	// Zone is the instance's zone.
	Zone string
	// Name is the instance's name.
	Name string
}

func (req *InstanceRequest) CheckAndSetDefaults() error {
	if len(req.ProjectID) == 0 {
		trace.BadParameter("projectID must be set")
	}
	if len(req.Zone) == 0 {
		trace.BadParameter("zone must be set")
	}
	if len(req.Name) == 0 {
		trace.BadParameter("name must be set")
	}
	return nil
}

// getHostKeys gets the SSH host keys from the VM, if available.
func (clt *instancesClient) getHostKeys(ctx context.Context, req *InstanceRequest) ([]ssh.PublicKey, error) {
	guestAttributes, err := clt.InstanceClient.GetGuestAttributes(ctx, &computepb.GetGuestAttributesInstanceRequest{
		Instance:  req.Name,
		Project:   req.ProjectID,
		Zone:      req.Zone,
		QueryPath: googleapi.String("hostkeys/"),
	})
	if err != nil {
		return nil, trace.Wrap(convertAPIError(err))
	}
	items := guestAttributes.GetQueryValue().GetItems()
	keys := make([]ssh.PublicKey, 0, len(items))
	var errors []error
	for _, item := range items {
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(fmt.Sprintf("%v %v", item.GetKey(), item.GetValue())))
		if err == nil {
			keys = append(keys, key)
		} else {
			errors = append(errors, err)
		}
	}
	return keys, trace.NewAggregate(errors...)
}

// GetInstance gets a GCP VM.
func (clt *instancesClient) GetInstance(ctx context.Context, req *InstanceRequest) (*Instance, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := clt.InstanceClient.Get(ctx, &computepb.GetInstanceRequest{
		Instance: req.Name,
		Project:  req.ProjectID,
		Zone:     req.Zone,
	})
	if err != nil {
		return nil, trace.Wrap(convertAPIError(err))
	}
	inst := toInstance(resp, req.ProjectID)
	inst.ProjectID = req.ProjectID

	hostKeys, err := clt.getHostKeys(ctx, req)
	if err == nil {
		inst.hostKeys = hostKeys
	} else if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return inst, nil
}

// SSHKeyRequest contains parameters to add/removed SSH keys from an instance.
type SSHKeyRequest struct {
	// Instance is the instance to add/remove keys form.
	Instance *Instance
	// PublicKey is the key to add. Ignored when removing a key.
	PublicKey ssh.PublicKey
	// Expires is the expiration time of the key. Ignored when removing a key.
	Expires time.Time
}

func (req *SSHKeyRequest) CheckAndSetDefaults() error {
	if req.Instance == nil {
		return trace.BadParameter("instance not set")
	}
	if req.Expires.IsZero() {
		// Default to 10 minutes to give plenty of time to install Teleport.
		req.Expires = time.Now().Add(10 * time.Minute)
	}
	return nil
}

// formatSSHKey formats a public key to add to a GCP VM.
func formatSSHKey(pubKey ssh.PublicKey, expires time.Time) string {
	const iso8601Format = "2006-01-02T15:04:05-0700"
	return fmt.Sprintf(`%s:%s %s google-ssh {"userName":%q,"expireOn":%q}`,
		sshUser,
		pubKey.Type(),
		base64.StdEncoding.EncodeToString(bytes.TrimSpace(pubKey.Marshal())),
		sshUser,
		expires.Format(iso8601Format),
	)
}

// sshKeyName is the name of the key in an instance's metadata that holds SSH
// keys.
const sshKeyName = "ssh-keys"

func addSSHKey(meta *computepb.Metadata, pubKey ssh.PublicKey, expires time.Time) {
	var sshKeyItem *computepb.Items
	for _, item := range meta.GetItems() {
		if item.GetKey() == sshKeyName {
			sshKeyItem = item
			break
		}
	}
	if sshKeyItem == nil {
		sshKeyItem = &computepb.Items{Key: googleapi.String(sshKeyName)}
		meta.Items = append(meta.Items, sshKeyItem)
	}

	var existingKeys []string
	if rawKeys := sshKeyItem.GetValue(); rawKeys != "" {
		existingKeys = strings.Split(rawKeys, "\n")
	}
	existingKeys = append(existingKeys, formatSSHKey(pubKey, expires))
	newKeys := strings.Join(existingKeys, "\n")
	sshKeyItem.Value = &newKeys
}

// AddSSHKey adds an SSH key to a GCP VM's metadata.
func (clt *instancesClient) AddSSHKey(ctx context.Context, req *SSHKeyRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if req.PublicKey == nil {
		return trace.BadParameter("public key not set")
	}
	addSSHKey(req.Instance.metadata, req.PublicKey, req.Expires)
	op, err := clt.InstanceClient.SetMetadata(ctx, &computepb.SetMetadataInstanceRequest{
		Instance:         req.Instance.Name,
		MetadataResource: req.Instance.metadata,
		Project:          req.Instance.ProjectID,
		Zone:             req.Instance.Zone,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := op.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func removeSSHKey(meta *computepb.Metadata) {
	for _, item := range meta.GetItems() {
		if item.GetKey() != sshKeyName {
			continue
		}
		existingKeys := strings.Split(item.GetValue(), "\n")
		newKeys := make([]string, 0, len(existingKeys))
		for _, key := range existingKeys {
			if !strings.HasPrefix(key, sshUser) {
				newKeys = append(newKeys, key)
			}
		}
		item.Value = googleapi.String(strings.TrimSpace(strings.Join(newKeys, "\n")))
		return
	}
}

// RemoveSSHKey removes an SSH key from a GCP VM's metadata.
func (clt *instancesClient) RemoveSSHKey(ctx context.Context, req *SSHKeyRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	removeSSHKey(req.Instance.metadata)
	op, err := clt.InstanceClient.SetMetadata(ctx, &computepb.SetMetadataInstanceRequest{
		Instance:         req.Instance.Name,
		MetadataResource: req.Instance.metadata,
		Project:          req.Instance.ProjectID,
		Zone:             req.Instance.Zone,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := op.Wait(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RunCommandRequest contains parameters for running a command on an instance.
type RunCommandRequest struct {
	// Client is the instance client to use.
	Client InstancesClient
	// InstanceRequest is the set of parameters identifying the instance.
	InstanceRequest
	// Script is the script to execute.
	Script string
	// SSHPort is the ssh server port to connect to. Defaults to 22.
	SSHPort string

	dialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

func (req *RunCommandRequest) CheckAndSetDefaults() error {
	if err := req.InstanceRequest.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if req.Script == "" {
		return trace.BadParameter("script must be set")
	}
	if req.SSHPort == "" {
		req.SSHPort = "22"
	}
	if req.dialContext == nil {
		dialer := net.Dialer{
			Timeout: sshDefaultTimeout,
		}
		req.dialContext = dialer.DialContext
	}
	return nil
}

func generateKeyPair() (ssh.Signer, ssh.PublicKey, error) {
	rawPriv, rawPub, err := native.GenerateKeyPair()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer, err := ssh.ParsePrivateKey(rawPriv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(rawPub)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return signer, publicKey, nil
}

// RunCommand runs a command on an instance.
func RunCommand(ctx context.Context, req *RunCommandRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Generate keys and add them to the instance.
	signer, publicKey, err := generateKeyPair()
	if err != nil {
		return trace.Wrap(err)
	}
	instance, err := req.Client.GetInstance(ctx, &req.InstanceRequest)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(instance.hostKeys) == 0 {
		return trace.NotFound(`Instance %v is missing host keys. Did you enable guest attributes on the instance?
https://cloud.google.com/solutions/connecting-securely#storing_host_keys_by_enabling_guest_attributes`, req.Name)
	}
	var ipAddrs []string
	if instance.externalIPAddress != "" {
		ipAddrs = append(ipAddrs, instance.externalIPAddress)
	}
	if instance.internalIPAddress != "" {
		ipAddrs = append(ipAddrs, instance.internalIPAddress)
	}
	if len(ipAddrs) == 0 {
		return trace.NotFound("Instance %v is missing an IP address.", req.Name)
	}
	keyReq := &SSHKeyRequest{
		Instance:  instance,
		PublicKey: publicKey,
	}
	if err := req.Client.AddSSHKey(ctx, keyReq); err != nil {
		return trace.Wrap(err)
	}

	// Clean up the key when we're done (if this fails, the key will
	// automatically expire after 10 minutes).
	defer func() {
		var err error
		// Fetch the instance first to get the most up-to-date metadata hash.
		if keyReq.Instance, err = req.Client.GetInstance(ctx, &req.InstanceRequest); err != nil {
			logrus.WithError(err).Warn("Error fetching instance.")
			return
		}
		if err := req.Client.RemoveSSHKey(ctx, keyReq); err != nil {
			logrus.WithError(err).Warn("Error deleting SSH Key.")
		}
	}()

	// Configure the SSH client.
	callback, err := sshutils.HostKeyCallback(instance.hostKeys, true)
	if err != nil {
		return trace.Wrap(err)
	}
	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: callback,
	}

	var errs []error
	for _, ip := range ipAddrs {
		addr := net.JoinHostPort(ip, req.SSHPort)
		stdout, stderr, err := sshutils.RunSSH(ctx, addr, req.Script, config, sshutils.WithDialer(req.dialContext))
		logrus.Debug(string(stdout))
		logrus.Debug(string(stderr))
		if err == nil {
			return nil
		}

		// An exit error means the connection was successful, so don't try another address.
		if errors.Is(err, &ssh.ExitError{}) {
			return trace.Wrap(err)
		}
		errs = append(errs, err)
	}

	err = trace.NewAggregate(errs...)
	logrus.WithError(err).Debug("Command exited with error.")
	return err
}
