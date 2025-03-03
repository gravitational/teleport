## Contributing

### Adding support for new Teleport resources

The steps to add support for new Teleport types to the operator are as follows:

#### Make sure the existing CRDs are up to date

In a clean repo before making any changes:

1. Run `make manifests`.
2. Run `make -C crdgen update-protos`.
3. Run `make -C crdgen update-snapshot`.

Make sure everything looks sane and commit any changes (there will be some if
other .proto files used to generate the CRDs have changed).

#### Generate the new CRD

1. Add the type name to the `resources` list in `crdgen/handlerequest.go`.
2. Add the proto file to the `PROTOS` list in `Makefile` if it is not
   already present. Also add it to the `PROTOS` list in `crdgen/Makefile`.
3. Run `make manifests` to generate the CRD.
4. Run `make crdgen-test`. This will should fail if your new CRD is generated.
   Update the test snapshots with `make -C crdgen update-snapshot`

#### Create a "scheme" defining Go types to match the CRD

Add a type for your resource under `apis/resources/<version>`.
Follow the same patterns of existing types in those packages.
If there is no existing directory for the current version of your resource,
create one, and don't forget to in include a `groupversion_info.go` file copied
from one of the existing version directories (with the correct version).
Do not forget to include the //+kubebuilder comments in these files.

#### Create a reconciler for the new resource type

Create a reconciler under the `controllers/resources` path.
Follow the same patterns of existing reconcilers in those packages.
Use the generic TeleportResourceReconciler if possible, that way you only have
to implement CRUD methods for your resource.

Write unit tests for your reconciler. Use the generic `ResourceCreationTest`,
`ResourceDeletionDriftTest`, and `ResourceUpdateTest` helpers to get baseline
coverage.

Update the `defaultTeleportServiceConfig` teleport role in
`controllers/resources/testlib/env.go` with any new required permissions.

#### Register your reconciler and scheme

In `controllers/resources/setup.go` instantiate your
controller and register it with the controller-runtime manager.
Follow the pattern of existing resources which instantiate the reconciler and
call the `SetupWithManager(mgr)` method.

If you added a new version under `apis/resources/`, make sure the scheme for
your resource version is added to the root `scheme` with a call like
`resourcesvX.AddToScheme(scheme)` in the `init()` function.

#### Add RBAC permissions for the new resource type

Add Kubernetes RBAC permissions to allow the operator to work with the resources
on the Kubernetes side.
The cluster role spec is found in  `../../examples/chart/teleport-cluster/templates/auth/config.yaml`.

Update the RBAC permissions in `hack/fixture-operator-role.yaml` to update
operator the role used for debugging.

### Debugging tips

#### Debugging in tests

You can set the `OPERATOR_TEST_TELEPORT_ADDR` environment variable to run the
controller tests against a live teleport cluster instead of a temporary one
created in the test. It will use your local `tsh` credentials to authenticate,
so make sure the role of your logged-in user has sufficient permissions.

#### Debugging against remote Kubernetes and Teleport clusters

The operator can run against both remote Teleport and Kubernetes clusters,
you'll need:

- a kubernetes cluster with the CRDs installed
- a Teleport cluster

To set up the operator:

- Make sure CRDs are deployed in the Kuberntes cluster. If they're not, you can deploy them with:
  ```shell
  helm install crds ../../examples/chart/teleport-cluster/charts/teleport-operator/ --set enabled=false --set installCRDs=always
  ```
- Open a proxy to the Kube API authenticating requests as yourself by following
  [the kube-agent-updater DEBUG guide](./../kube-agent-updater/DEBUG.md)
- Create the Teleport operator role
  ```shell
  tctl create -f ./hack/fixture-operator-role.yaml
  ```
- Create the Teleport bot role and user
  ```yaml
  tctl create -f ./hack/fixture-operator-bot.yaml
  ```
- Create the Teleport bot token
  ```yaml
  tctl create -f ./hack/fixture-operator-token.yaml
  ```

**WARNING**: the static token gets consumed on first use, restarting the operator
requires create a new token AND deleting the certificate generation label on
the bot user. DO NOT USE this join method in production.

Finally, run the operator. For example:

```shell
make build

./bin/manager \
  -join-method token \
  -token operator-token \
  -auth-server <TELEPORT_ADDR> \
  -kubeconfig <PATH_TO_TEMP_KUBECONFIG> \
  -namespace <NAMESPACE_WHERE_CRS_ARE>
```
