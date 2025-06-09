package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var userTask = resource{
	getHandler:    getUserTask,
	createHandler: createUserTask,
	updateHandler: updateUserTask,
	deleteHandler: deleteUserTask,
}

func createUserTask(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	resource, err := services.UnmarshalUserTask(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.UserTasksServiceClient()
	if opts.force {
		if _, err := c.UpsertUserTask(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user task %q has been updated\n", resource.GetMetadata().GetName())
	} else {
		if _, err := c.CreateUserTask(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("user task %q has been created\n", resource.GetMetadata().GetName())
	}

	return nil
}

func getUserTask(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	userTasksClient := client.UserTasksClient()
	if ref.Name != "" {
		uit, err := userTasksClient.GetUserTask(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewUserTaskCollection([]*usertasksv1.UserTask{uit}), nil
	}

	var tasks []*usertasksv1.UserTask
	nextToken := ""
	for {
		resp, token, err := userTasksClient.ListUserTasks(ctx, 0 /* default size */, nextToken, &usertasksv1.ListUserTasksFilters{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tasks = append(tasks, resp...)

		if token == "" {
			break
		}
		nextToken = token
	}
	return collections.NewUserTaskCollection(tasks), nil
}

func updateUserTask(ctx context.Context, client *authclient.Client, resource services.UnknownResource, opts createOpts) error {
	in, err := services.UnmarshalUserTask(resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.UserTasksServiceClient().UpdateUserTask(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user task %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func deleteUserTask(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.UserTasksServiceClient().DeleteUserTask(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("user task %q has been deleted\n", ref.Name)
	return nil
}
