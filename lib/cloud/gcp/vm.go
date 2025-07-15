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
	"log/slog"
	"net"
	"slices"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/cryptosuites"
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
	var apiError *apierror.APIError
	if errors.As(err, &apiError) {
		if code := apiError.HTTPCode(); code != -1 {
			return trace.ReadError(code, []byte(apiError.Reason()))
		} else if apiError.GRPCStatus() != nil {
			return trail.FromGRPC(apiError)
		}
	}
	return err
}

// InstancesClient is a client to interact with GCP VMs.
type InstancesClient interface {
	// ListInstances lists the GCP VMs that belong to the given project and
	// zone.
	// zone supports wildcard "*".
	ListInstances(ctx context.Context, projectID, zone string) ([]*gcpimds.Instance, error)
	// StreamInstances streams the GCP VMs that belong to the given project and
	// zone.
	// zone supports wildcard "*".
	StreamInstances(ctx context.Context, projectID, zone string) stream.Stream[*gcpimds.Instance]
	// GetInstance gets a GCP VM.
	GetInstance(ctx context.Context, req *gcpimds.InstanceRequest) (*gcpimds.Instance, error)
	// AddSSHKey adds an SSH key to a GCP VM's metadata.
	AddSSHKey(ctx context.Context, req *SSHKeyRequest) error
	// RemoveSSHKey removes an SSH key from a GCP VM's metadata.
	RemoveSSHKey(ctx context.Context, req *SSHKeyRequest) error
	// GetInstanceTags gets the GCP tags associated with an instance (which are
	// distinct from its labels). It is separate from GetInstance because fetching
	// tags requires its own permissions.
	GetInstanceTags(ctx context.Context, req *gcpimds.InstanceRequest) (map[string]string, error)
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

func toInstance(origInstance *computepb.Instance, projectID string) *gcpimds.Instance {
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

	items := make(map[string]string, len(origInstance.GetMetadata().GetItems()))
	for _, item := range origInstance.GetMetadata().GetItems() {
		if item.Key == nil {
			continue
		}
		items[item.GetKey()] = item.GetValue()
	}

	inst := &gcpimds.Instance{
		Name:              origInstance.GetName(),
		ProjectID:         projectID,
		Zone:              zone,
		Labels:            origInstance.GetLabels(),
		InternalIPAddress: internalIP,
		ExternalIPAddress: externalIP,
		Fingerprint:       origInstance.GetMetadata().GetFingerprint(),
		MetadataItems:     items,
	}
	// GCP VMs can have at most one service account.
	if len(origInstance.ServiceAccounts) > 0 {
		inst.ServiceAccount = origInstance.ServiceAccounts[0].GetEmail()
	}
	return inst
}

func toInstances(origInstances []*computepb.Instance, projectID string) []*gcpimds.Instance {
	instances := make([]*gcpimds.Instance, 0, len(origInstances))
	for _, inst := range origInstances {
		instances = append(instances, toInstance(inst, projectID))
	}
	return instances
}

// ListInstances lists the GCP VMs that belong to the given project and
// zone.
// zone supports wildcard "*".
func (clt *instancesClient) ListInstances(ctx context.Context, projectID, zone string) ([]*gcpimds.Instance, error) {
	instances, err := stream.Collect(clt.StreamInstances(ctx, projectID, zone))
	return instances, trace.Wrap(err)
}

func (clt *instancesClient) StreamInstances(ctx context.Context, projectID, zone string) stream.Stream[*gcpimds.Instance] {
	if len(projectID) == 0 {
		return stream.Fail[*gcpimds.Instance](trace.BadParameter("projectID must be set"))
	}
	if len(zone) == 0 {
		return stream.Fail[*gcpimds.Instance](trace.BadParameter("location must be set"))
	}

	var getInstances func() ([]*gcpimds.Instance, error)

	if zone == types.Wildcard {
		it := clt.InstanceClient.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{
			Project: projectID,
		})
		getInstances = func() ([]*gcpimds.Instance, error) {
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
		getInstances = func() ([]*gcpimds.Instance, error) {
			resp, err := it.Next()
			if resp == nil {
				return nil, trace.Wrap(convertAPIError(err))
			}
			return []*gcpimds.Instance{toInstance(resp, projectID)}, trace.Wrap(convertAPIError(err))
		}
	}

	return stream.PageFunc(func() ([]*gcpimds.Instance, error) {
		instances, err := getInstances()
		if errors.Is(err, iterator.Done) {
			return instances, io.EOF
		}
		return instances, trace.Wrap(err)
	})
}

// getHostKeys gets the SSH host keys from the VM, if available.
func (clt *instancesClient) getHostKeys(ctx context.Context, req *gcpimds.InstanceRequest) ([]ssh.PublicKey, error) {
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
	var errs []error
	for _, item := range items {
		key, _, _, _, err := ssh.ParseAuthorizedKey(fmt.Appendf(nil, "%v %v", item.GetKey(), item.GetValue()))
		if err == nil {
			keys = append(keys, key)
		} else {
			errs = append(errs, err)
		}
	}
	return keys, trace.NewAggregate(errs...)
}

// GetInstance gets a GCP VM.
func (clt *instancesClient) GetInstance(ctx context.Context, req *gcpimds.InstanceRequest) (*gcpimds.Instance, error) {
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

	if !req.WithoutHostKeys {
		hostKeys, err := clt.getHostKeys(ctx, req)
		if err == nil {
			inst.HostKeys = hostKeys
		} else if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	return inst, nil
}

func (clt *instancesClient) getTagBindingsClient(ctx context.Context, zone string) (*resourcemanager.TagBindingsClient, error) {
	var opts []option.ClientOption
	if zone != "" {
		endpoint := zone + "-cloudresourcemanager.googleapis.com:443"
		opts = append(opts, option.WithEndpoint(endpoint))
	}
	client, err := resourcemanager.NewTagBindingsClient(ctx, opts...)
	return client, trace.Wrap(convertAPIError(err))
}

// GetInstanceTags gets the GCP tags for the instance.
func (clt *instancesClient) GetInstanceTags(ctx context.Context, req *gcpimds.InstanceRequest) (map[string]string, error) {
	tagClient, err := clt.getTagBindingsClient(ctx, req.Zone)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	it := tagClient.ListEffectiveTags(ctx, &resourcemanagerpb.ListEffectiveTagsRequest{
		Parent: fmt.Sprintf(
			"//compute.googleapis.com/projects/%s/zones/%s/instances/%d",
			req.ProjectID, req.Zone, req.ID,
		),
	})

	tags := make(map[string]string)
	for {
		resp, err := it.Next()
		if err != nil {
			if errors.Is(err, iterator.Done) {
				return tags, nil
			}
			return nil, trace.Wrap(convertAPIError(err))
		}
		// Tag value is in the form <project-name>/<key>/<value>
		fields := strings.Split(resp.GetNamespacedTagValue(), "/")
		k := fields[len(fields)-2]
		v := fields[len(fields)-1]
		tags[k] = v
	}
}

// SSHKeyRequest contains parameters to add/removed SSH keys from an instance.
type SSHKeyRequest struct {
	// Instance is the instance to add/remove keys form.
	Instance *gcpimds.Instance
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

func addSSHKey(instance *gcpimds.Instance, pubKey ssh.PublicKey, expires time.Time) {
	var existingKeys []string
	if keys, ok := instance.MetadataItems[sshKeyName]; ok {
		existingKeys = strings.Split(keys, "\n")
	}

	existingKeys = append(existingKeys, formatSSHKey(pubKey, expires))
	instance.MetadataItems[sshKeyName] = strings.Join(existingKeys, "\n")
}

// AddSSHKey adds an SSH key to a GCP VM's metadata.
func (clt *instancesClient) AddSSHKey(ctx context.Context, req *SSHKeyRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if req.PublicKey == nil {
		return trace.BadParameter("public key not set")
	}
	addSSHKey(req.Instance, req.PublicKey, req.Expires)

	op, err := clt.InstanceClient.SetMetadata(ctx, &computepb.SetMetadataInstanceRequest{
		Instance:         req.Instance.Name,
		MetadataResource: convertInstanceMetadata(req.Instance),
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

func removeSSHKey(instance *gcpimds.Instance) {
	keys, ok := instance.MetadataItems[sshKeyName]
	if !ok {
		return
	}

	existingKeys := strings.Split(keys, "\n")
	newKeys := make([]string, 0, len(existingKeys))
	for _, key := range existingKeys {
		if !strings.HasPrefix(key, sshUser) {
			newKeys = append(newKeys, key)
		}
	}

	instance.MetadataItems[sshKeyName] = strings.TrimSpace(strings.Join(newKeys, "\n"))
}

func convertInstanceMetadata(i *gcpimds.Instance) *computepb.Metadata {
	items := make([]*computepb.Items, 0, len(i.MetadataItems))
	for k, v := range i.MetadataItems {
		items = append(items, &computepb.Items{Key: googleapi.String(k), Value: googleapi.String(v)})
	}

	return &computepb.Metadata{
		Fingerprint: googleapi.String(i.Fingerprint),
		Items:       items,
	}
}

// RemoveSSHKey removes an SSH key from a GCP VM's metadata.
func (clt *instancesClient) RemoveSSHKey(ctx context.Context, req *SSHKeyRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	removeSSHKey(req.Instance)

	op, err := clt.InstanceClient.SetMetadata(ctx, &computepb.SetMetadataInstanceRequest{
		Instance:         req.Instance.Name,
		MetadataResource: convertInstanceMetadata(req.Instance),
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
	gcpimds.InstanceRequest
	// Script is the script to execute.
	Script string
	// SSHPort is the ssh server port to connect to. Defaults to 22.
	SSHPort string
	// SSHKeyAlgo is the algorithm to use for generated SSH keys.
	SSHKeyAlgo cryptosuites.Algorithm

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
	if req.SSHKeyAlgo == cryptosuites.Algorithm(0) {
		return trace.BadParameter("ssh key algorithm must be set")
	}
	if req.dialContext == nil {
		dialer := net.Dialer{
			Timeout: sshDefaultTimeout,
		}
		req.dialContext = dialer.DialContext
	}
	return nil
}

func generateKeyPair(keyAlgo cryptosuites.Algorithm) (ssh.Signer, error) {
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(keyAlgo)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshSigner, err := ssh.NewSignerFromSigner(signer)
	return sshSigner, trace.Wrap(err)
}

// RunCommand runs a command on an instance.
func RunCommand(ctx context.Context, req *RunCommandRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Generate keys and add them to the instance.
	signer, err := generateKeyPair(req.SSHKeyAlgo)
	if err != nil {
		return trace.Wrap(err)
	}
	instance, err := req.Client.GetInstance(ctx, &req.InstanceRequest)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(instance.HostKeys) == 0 {
		return trace.NotFound(`Instance %v is missing host keys. Did you enable guest attributes on the instance?
https://cloud.google.com/solutions/connecting-securely#storing_host_keys_by_enabling_guest_attributes`, req.Name)
	}
	var ipAddrs []string
	if instance.ExternalIPAddress != "" {
		ipAddrs = append(ipAddrs, instance.ExternalIPAddress)
	}
	if instance.InternalIPAddress != "" {
		ipAddrs = append(ipAddrs, instance.InternalIPAddress)
	}
	if len(ipAddrs) == 0 {
		return trace.NotFound("Instance %v is missing an IP address.", req.Name)
	}
	keyReq := &SSHKeyRequest{
		Instance:  instance,
		PublicKey: signer.PublicKey(),
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
			slog.WarnContext(ctx, "Error fetching instance", "error", err)
			return
		}
		if err := req.Client.RemoveSSHKey(ctx, keyReq); err != nil {
			slog.WarnContext(ctx, "Error deleting SSH Key", "error", err)
		}
	}()

	// Configure the SSH client.
	callback, err := sshutils.HostKeyCallback(instance.HostKeys, true)
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

	loggerWithVMMetadata := slog.With(
		"project_id", req.ProjectID,
		"zone", req.Zone,
		"vm_name", req.Name,
		"ips", ipAddrs,
	)

	var errs []error
	for _, ip := range ipAddrs {
		addr := net.JoinHostPort(ip, req.SSHPort)
		stdout, stderr, err := sshutils.RunSSH(ctx, addr, req.Script, config, sshutils.WithDialer(req.dialContext))
		if err == nil {
			return nil
		}

		// An exit error means the connection was successful, so don't try another address.
		if errors.Is(err, &ssh.ExitError{}) {
			loggerWithVMMetadata.ErrorContext(ctx, "Installing teleport in GCP VM failed after connecting",
				"ip", ip,
				"error", err,
				"stdout", string(stdout),
				"stderr", string(stderr),
			)
			return trace.Wrap(err)
		}
		errs = append(errs, err)
	}

	err = trace.NewAggregate(errs...)
	loggerWithVMMetadata.ErrorContext(ctx, "Installing teleport in GCP VM failed", "error", err)
	return err
}
