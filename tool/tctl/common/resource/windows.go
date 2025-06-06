package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getWindowsDesktopService(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	services, err := client.GetWindowsDesktopServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewWindowsDesktopServiceCollection(services), nil
	}

	var out []types.WindowsDesktopService
	for _, service := range services {
		if service.GetName() == rc.ref.Name {
			out = append(out, service)
		}
	}
	if len(out) == 0 {
		return nil, trace.NotFound("Windows desktop service %q not found", rc.ref.Name)
	}
	return collections.NewWindowsDesktopServiceCollection(out), nil
}

func (rc *ResourceCommand) deleteWindowsDesktopService(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteWindowsDesktopService(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("windows desktop service %q has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) getWindowsDesktop(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	desktops, err := client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewWindowsDesktopCollection(desktops), nil
	}

	var out []types.WindowsDesktop
	for _, desktop := range desktops {
		if desktop.GetName() == rc.ref.Name {
			out = append(out, desktop)
		}
	}
	if len(out) == 0 {
		return nil, trace.NotFound("Windows desktop %q not found", rc.ref.Name)
	}
	return collections.NewWindowsDesktopCollection(out), nil
}

func (rc *ResourceCommand) createWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	wd, err := services.UnmarshalWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.UpsertWindowsDesktop(ctx, wd); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("windows desktop %q has been updated\n", wd.GetName())
	return nil
}

func (rc *ResourceCommand) deleteWindowsDesktop(ctx context.Context, client *authclient.Client) error {
	desktops, err := client.GetWindowsDesktops(ctx,
		types.WindowsDesktopFilter{Name: rc.ref.Name})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(desktops) == 0 {
		return trace.NotFound("no desktops with name %q were found", rc.ref.Name)
	}
	deleted := 0
	var errs []error
	for _, desktop := range desktops {
		if desktop.GetName() == rc.ref.Name {
			if err = client.DeleteWindowsDesktop(ctx, desktop.GetHostID(), rc.ref.Name); err != nil {
				errs = append(errs, err)
				continue
			}
			deleted++
		}
	}
	if deleted == 0 {
		errs = append(errs,
			trace.Errorf("failed to delete any desktops with the name %q, %d were found",
				rc.ref.Name, len(desktops)))
	}
	fmts := "%d windows desktops with name %q have been deleted"
	if err := trace.NewAggregate(errs...); err != nil {
		fmt.Printf(fmts+" with errors while deleting\n", deleted, rc.ref.Name)
		return err
	}
	fmt.Printf(fmts+"\n", deleted, rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) getDynamicWindowsDesktop(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	dynamicDesktopClient := client.DynamicDesktopClient()
	if rc.ref.Name != "" {
		desktop, err := dynamicDesktopClient.GetDynamicWindowsDesktop(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewDynamicWindowsDesktopCollection([]types.DynamicWindowsDesktop{desktop}), nil
	}

	pageToken := ""
	desktops := make([]types.DynamicWindowsDesktop, 0, 100)
	for {
		d, next, err := dynamicDesktopClient.ListDynamicWindowsDesktops(ctx, 100, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if rc.ref.Name == "" {
			desktops = append(desktops, d...)
		} else {
			for _, desktop := range desktops {
				if desktop.GetName() == rc.ref.Name {
					desktops = append(desktops, desktop)
				}
			}
		}
		pageToken = next
		if next == "" {
			break
		}
	}

	return collections.NewDynamicWindowsDesktopCollection(desktops), nil
}

func (rc *ResourceCommand) createDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	wd, err := services.UnmarshalDynamicWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	dynamicDesktopClient := client.DynamicDesktopClient()
	if _, err := dynamicDesktopClient.CreateDynamicWindowsDesktop(ctx, wd); err != nil {
		if trace.IsAlreadyExists(err) {
			if !rc.force {
				return trace.AlreadyExists("application %q already exists", wd.GetName())
			}
			if _, err := dynamicDesktopClient.UpsertDynamicWindowsDesktop(ctx, wd); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("dynamic windows desktop %q has been updated\n", wd.GetName())
			return nil
		}
		return trace.Wrap(err)
	}

	fmt.Printf("dynamic windows desktop %q has been updated\n", wd.GetName())
	return nil
}

func (rc *ResourceCommand) updateDynamicWindowsDesktop(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	wd, err := services.UnmarshalDynamicWindowsDesktop(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	dynamicDesktopClient := client.DynamicDesktopClient()
	if _, err := dynamicDesktopClient.UpdateDynamicWindowsDesktop(ctx, wd); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("dynamic windows desktop %q has been updated\n", wd.GetName())
	return nil
}

func (rc *ResourceCommand) deleteDynamicWindowsDesktop(ctx context.Context, client *authclient.Client) error {
	if err := client.DynamicDesktopClient().DeleteDynamicWindowsDesktop(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("dynamic windows desktop %q has been deleted\n", rc.ref.Name)
	return nil
}
