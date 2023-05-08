# Checkout Teleport action

This `checkout-teleport` action is used to checkout the Teleport
OSS repository and optionally a Teleport Enterprise repository. It has
flexibility in specifying which Teleport OSS repository is to be checked
out (usually the main Teleport repo, but sometimes teleport-private) as
well as which branch of each OSS and Enterprise to be checked out.

It currently is only called from workflows in this teleport.e
repository, so that is the only repository that can be used for checking
out teleport.e. If a private Teleport OSS repository is to be checked
out, a GitHub App must be specified with the `repo-access-app-id` and
`repo-access-app-key` input parameters, which will be used to get a
token to access the private repository. Ensure the referenced
application has appropriate permission to read that repository.

See the [`action.yml`](action.yml) file for the input parameters, their
types, defaults, optionality and description.

