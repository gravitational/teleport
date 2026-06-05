package accessgraph

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// reconcileResults reconciles two Resources objects and returns the operations
// required to reconcile them into the new state.
// It returns two EntraResourceList objects, one for resources to upsert and one
// for resources to delete.
func reconcileResults(old *resources, new *resources) (upsert, delete *accessgraphv1alpha.EntraResourceList) {
	upsert, delete = &accessgraphv1alpha.EntraResourceList{}, &accessgraphv1alpha.EntraResourceList{}

	for _, results := range []*reconcileIntermediateResult{
		reconcileApplications(old.Applications, new.Applications),
	} {
		upsert.SetResources(append(upsert.GetResources(), results.upsert.GetResources()...))
		delete.SetResources(append(delete.GetResources(), results.delete.GetResources()...))
	}

	return upsert, delete
}

type reconcileIntermediateResult struct {
	upsert, delete *accessgraphv1alpha.EntraResourceList
}

func reconcileApplications(old []*accessgraphv1alpha.EntraApplication, new []*accessgraphv1alpha.EntraApplication) *reconcileIntermediateResult {
	upsert, delete := &accessgraphv1alpha.EntraResourceList{}, &accessgraphv1alpha.EntraResourceList{}

	toAdd, toRemove := reconcile(old, new, func(app *accessgraphv1alpha.EntraApplication) string {
		return app.GetAppId()
	})

	for _, app := range toAdd {
		upsert.SetResources(append(upsert.GetResources(), &accessgraphv1alpha.EntraResource{
			Resource: &accessgraphv1alpha.EntraResource_Application{
				Application: app,
			},
		}))
	}
	for _, app := range toRemove {
		delete.SetResources(append(delete.GetResources(), &accessgraphv1alpha.EntraResource{
			Resource: &accessgraphv1alpha.EntraResource_Application{
				Application: app,
			},
		}))
	}
	return &reconcileIntermediateResult{upsert, delete}
}

func reconcile[T protoreflect.ProtoMessage](old []T, new []T, key func(T) string) (upsert, delete []T) {
	if len(old) == 0 {
		return new, nil
	}
	if len(new) == 0 {
		return nil, old
	}

	oldMap := make(map[string]T, len(old))
	for _, item := range old {
		oldMap[key(item)] = item
	}

	newMap := make(map[string]T, len(new))
	for _, item := range new {
		newMap[key(item)] = item
	}

	for _, item := range new {
		if oldItem, ok := oldMap[key(item)]; !ok || !proto.Equal(oldItem, item) {
			upsert = append(upsert, item)
		}
	}
	for _, item := range old {
		if _, ok := newMap[key(item)]; !ok {
			delete = append(delete, item)
		}
	}
	return upsert, delete
}
