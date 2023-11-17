This directory contains test licenses for Teleport enterprise.

## Existing licenses

* `license-enterprise.pem` - legacy enterprise license.
* `license-pro.pem` - legacy pro (with reporting enabled) license.
* `license-all-features.pem` - license with Kubernetes, app and database access enabled.
* `license-no-features.pem` - license with Kubernetes, app and database access disabled.
* `license-cloud-ent.pem` - Teleport Cloud license
* `license-cloud-team.pem` - Teleport Cloud Team license

Cloud licenses are signed by [sales-center local development CA](https://github.com/gravitational/cloud/blob/a1c2e970b685d7d8a7ff37629655d2af535e4984/dev/salescenter.yaml#L46).

## Generating test license

To generate a new test license, use the generator program in:

https://github.com/gravitational/ops/tree/master/license

## Testing Team plan features

Some features are exclusive to the Teleport Team cloud plan. The license to use for that is `license-cloud-team.pem`.
Team plan tenants expect to fetch their `Features` from Sales Center.

To successfully launch a local Teleport instance with the team plan, please first [set up a local dev instance of Sales center](https://github.com/gravitational/cloud/blob/master/README.md#local-sales-center-development), then run teleport with:

```console
$ TELEPORT_CLOUD_HOSTPORT="https://api.localhost:3080" teleport start --insecure -c config.yaml
```

It is only required to go through this process once. Afterwards, the features will also be cached in the Teleport backend, and communication with Sales Center is optional.
