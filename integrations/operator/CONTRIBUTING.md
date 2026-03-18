## Contributing

### Adding support for new Teleport resources

The steps to add support for new Teleport types to the operator are as follows:

#### Make sure the existing CRDs are up to date

In a clean repo before making any changes:

1. Run `make generate`.

Make sure everything looks sane and commit any changes (there will be some if
other .proto files used to generate the CRDs have changed).

#### Generate the new CRD

1. Add the type name to the `resources` list in `crdgen/handlerequest.go`.

2. Add the proto file to the `PROTOS` list in `Makefile` if it is not
   already present.

3. Run `make crd-manifests-diff`. This will should output the new CRD file or
   the diff and exit with a 1 return code, indicating there are differences.

   Inspect the output to make sure it's only generating CRDs for the new
   resource(s) you added. If other CRDs are generated or changed, figure out why
   and plan to test any other resources modified. Alternativly, revert the
   changes you didn't make causing the CRDs to be modified.

4. Run `make crd` to generate the CRD and its documentation.

5. Run `make crd-manifests-diff` again. After running `make crd`, it should
   always show no differences and exit without error.

#### Create a "scheme" defining Go types to match the CRD

Add a type for your resource under `apis/resources/<version>`.
Follow the same patterns of existing types in those packages.
If there is no existing directory for the current version of your resource,
create one, and don't forget to in include a `groupversion_info.go` file copied
from one of the existing version directories (with the correct version).
Do not forget to include the //+kubebuilder comments in these files.

After adding the resource, run `make generate` to generate its `DeepCopy*`
methods.

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

- Grant the operator access to the Kubernetes resource in: `../../examples/chart/teleport-cluster/charts/teleport-operator/templates/role.yaml`.
- Grant the operator access to the Teleport resource in: `../../examples/chart/teleport-cluster/templates/auth/config.yaml`.
- Update the RBAC permissions in `hack/fixture-operator-role.yaml` to update operator the role used for debugging.

### Testing

#### Quick test with k3d (using a released Teleport image)

Run `make k3d-deploy` to deploy the operator alongside a released Teleport
version in a local k3d cluster.

Prerequisites:
- [k3d](https://k3d.io/) installed
- Docker running

```shell
# Create the k3d cluster
k3d cluster create k3s-default

# Build the operator and deploy everything
make k3d-deploy
```

#### Testing with a local Teleport build

If you need to test against a locally built Teleport (e.g. to test unreleased
features or feature-flagged changes), override `TELEPORT_IMAGE` and
`TELEPORT_IMAGE_VERSION`. The Makefile auto-detects a local build when
`TELEPORT_IMAGE` differs from the default registry image, and will import the
local image into k3d.

1. **Build the Teleport image** from the repo root. This builds all binaries
   inside a Docker buildbox and produces a local image:

   ```shell
   # From the repo root
   make image
   ```

   This creates an image like `teleport:19.0.0-dev-arm64`. By default, if you have the e/ directory available,
   you're going to need to also include the license when deploying your teleport pods.

   > **Troubleshooting local image build:**
   > If you're having issues running make image:
   >
   > Run `docker builder prune -af` to clear build cache entries.
   > If the buildbox is stale (e.g. Go/Rust version mismatches), you may need to rebuild it first with `make -C build.assets buildbox-centos7`.
   > 
   > Make sure your e ref is up to date. There may be issues with some thing being no longer needed in oss that is still in e/.
   >
   > Run `rustup override unset` if having issues with Rust or `make ensure-wasm-bindgen FORCE=true`. 
   >

2. **Create a k3d cluster** (if you don't already have one):

   ```shell
   k3d cluster create k3s-default
   ```

3. **Deploy with the local image:**

   For enterprise builds, create the license secret before deploying:

   ```shell
   export KUBECONFIG=$(k3d kubeconfig write k3s-default)
   kubectl create namespace test
   kubectl -n test create secret generic license \
     --from-file=license.pem=your-path-to-license-file
   ```

   ```shell
   cd integrations/operator

   # Enterprise local build (also set ENTERPRISE=1)
   make k3d-deploy \
     ENTERPRISE=1 \
     TELEPORT_IMAGE=teleport \
     TELEPORT_IMAGE_VERSION=19.0.0-dev-arm64
   ```

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
