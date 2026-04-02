This directory contains test licenses for Teleport enterprise.

## Existing licenses

- `license-all-features.pem` - license with Kubernetes, app, database and desktop access enabled.
- `license-cloud-ent.pem` - Teleport Cloud license
- `license-cloud-team.pem` - Teleport Cloud Team license

Cloud licenses are signed by [sales-center local development CA](https://github.com/gravitational/cloud/blob/a1c2e970b685d7d8a7ff37629655d2af535e4984/dev/salescenter.yaml#L46).

## Generating test license

To generate a new test license, use the generator program in the [Cloud](https://github.com/gravitational/cloud) repository: [generate_teleport_license](https://github.com/gravitational/cloud/tree/master/scripts/generate_teleport_license).

## Testing Team plan features

Some features are exclusive to the Teleport Team cloud plan. The license to use for that is `license-cloud-team.pem`.
Team plan tenants expect to fetch their `Features` from Sales Center.

To successfully launch a local Teleport instance with the team plan, please first [set up a local dev instance of Sales center](https://github.com/gravitational/cloud/blob/master/README.md#local-sales-center-development), then run teleport with:

```console
$ TELEPORT_CLOUD_HOSTPORT="https://api.localhost:3080" teleport start --insecure -c config.yaml
```

It is only required to go through this process once. Afterwards, the features will also be cached in the Teleport backend, and communication with Sales Center is optional.

## Testing EUB (Enterprise Usage Based) features

There are two types of EUB plans:

1. On-prem: `on-prem` licenses can be found under this directory named `license-eub-with-igs` (with Identity Governance Security enabled) and `license-eub-without-igs`.
2. Cloud: This ones more involved as you'll need salescenter running. It's similar process to testing Team (section above this) except that you'll need to generate your own license using the salescenter web UI. In the salescenter web UI, create account, invite yourself, sign up with the link, and then copy the license generated and stored in the psql backend.
