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
