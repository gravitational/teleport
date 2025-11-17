package resources

import (
	"context"
	"errors"
	"fmt"
	"io"

	cloudclusterv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

type cloudClusterCollection struct {
	cloudClusters []*cloudclusterv1pb.CloudCluster
}

func NewCloudClusterCollection(cloudClusters []*cloudclusterv1pb.CloudCluster) Collection {
	return &cloudClusterCollection{
		cloudClusters: cloudClusters,
	}
}

func (c *cloudClusterCollection) Resources() []types.Resource {
	resources := make([]types.Resource, 0, len(c.cloudClusters))

	for _, cc := range c.cloudClusters {
		resources = append(resources, types.ProtoResource153ToLegacy(cc))
	}

	return resources
}

func (c *cloudClusterCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, cc := range c.cloudClusters {
		t.AddRow([]string{
			cc.GetMetadata().GetName(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func cloudClusterHandler() Handler {
	return Handler{
		getHandler:    getCloudCluster,
		createHandler: createCloudCluster,
		// updateHandler: updateCloudCluster,
		deleteHandler: deleteCloudCluster,
		singleton:     false,
		mfaRequired:   true,
		description:   "A cloud cluster managed by Teleport",
	}
}

func getCloudCluster(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		// 	users, err := client.ListGithubConnectors(ctx)
		// 	if err != nil {
		// 		return nil, trace.Wrap(err)
		// 	}
		// 	return &cloudClusterCollection{users: users}, nil
		return nil, errors.New("name is missing")
	}
	// user, err := client.GetUser(ctx, ref.Name, opts.WithSecrets)
	cloudCluster, err := client.CloudClusterServiceClient().GetCloudCluster(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cloudClusterCollection{
		cloudClusters: []*cloudclusterv1pb.CloudCluster{
			cloudCluster,
		},
	}, nil
}

func createCloudCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	config, err := services.UnmarshalProtoResource[*cloudclusterv1pb.CloudCluster](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		_, err = client.CloudClusterServiceClient().UpsertCloudCluster(ctx, config)
	} else {
		_, err = client.CloudClusterServiceClient().CreateCloudCluster(ctx, config)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("cloud_cluster has been created")
	return nil
}

func deleteCloudCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	err := client.CloudClusterServiceClient().DeleteCloudCluster(ctx, ref.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("cloud_cluster has been deleted")
	return nil
}
