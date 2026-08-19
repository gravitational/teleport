# Checkout Teleport action

This `checkout-teleport` action is used to checkout Teleport. For Enterprise
builds, it supports both repository layouts: older revisions where `e` is a
submodule referencing the Teleport Enterprise repository, and newer revisions
where `e` is an ordinary directory embedded in the checked-out repository. It
has flexibility in specifying which Teleport repository is to be checked out
as well as which revision to use.

It currently is only called from workflows in this teleport.e
repository, so that is the only repository that can be used for checking
out teleport.e. If a private Teleport OSS repository is to be checked
out, a GitHub App must be specified with the `repo-access-app-id` and
`repo-access-app-key` input parameters, which will be used to get a
token to access the private repository. Ensure the referenced
application has appropriate permission to read that repository.

## Usage

See the [`action.yml`](action.yml) file for the input parameters, their
types, defaults, optionality and description.

The action should be named in the `uses:` field with the full GitHub
repository:

    uses: gravitational/teleport.e/.github/actions/checkout-teleport@master

It is not possible to use the action as a repository-local action as the
enterprise repository is not yet checked out - that is the purpose of
this action.

If you want to test changes to this action within the same PR that
requires those changes, you will need to temporarily change `@master` to
`@your/branch-name`. Make sure you change it back to `@master` before
merging.


## Examples

### Checkout Teleport master and its corresponding Enterprise repo

    ...
    steps:
    - name: Checkout Teleport
      uses: gravitational/teleport.e/.github/actions/checkout-teleport@master
      with:
        oss-teleport-ref: master
        edition: enterprise

If `e` is a submodule, the submodule ref in the Teleport repository at
`master` will be used because `ent-teleport-ref` is not specified. If `e` is
an ordinary directory, its contents from the Teleport checkout will be used
directly.

### Checkout Teleport OSS only, master branch

    ...
    steps:
    - name: Checkout Teleport
      uses: gravitational/teleport.e/.github/actions/checkout-teleport@master
      with:
        oss-teleport-ref: master
        edition: oss

### Checkout teleport-private OSS repo with Enterprise

    ...
    steps:
    - name: Checkout Teleport
      uses: gravitational/teleport.e/.github/actions/checkout-teleport@master
      with:
        oss-teleport-repo: gravitational/teleport-private
        oss-teleport-ref: master
        edition: enterprise
        repo-access-app-id: ${{ secrets.DEPLOY_APP_ID }}
        repo-access-app-key: ${{ secrets.DEPLOY_APP_PRIVATE_KEY }}

This requires that a GitHub App be installed in the `gravitational/teleport.e`
repo with the App ID and App private key specified in the secrets
`DEPLOY_APP_ID` and `DEPLOY_APP_KEY`. This App installation must have
`repo:read` on the repository specified in the `oss-teleport-repo` input
(`gravitational/teleport-private` in this instance). It will be used to get a
temporary token to access the `gravitational/teleport-private` repository.

### Checkout Teleport branch corresponding to Enterprise PR base branch

    ...
    steps:
    - name: Checkout Teleport
      uses: gravitational/teleport.e/.github/actions/checkout-teleport@master
      with:
        oss-teleport-ref: ${{ github.base_ref }} # Only availale on PRs
        edition: enterprise
        ent-teleport-ref: ${{ github.ref }}
