---
authors: Fred Heinecke (fred.heinecke@goteleport.com)
state: implemented
---

# RFD 58 - Package Distribution

## What

Hosting and configuration of APT and YUM repositories.

## Why

Currently we are building .deb and .rpm file in Teleport's Drone pipeline via reprepro and createrepo, then publishing to AWS S3. Between the tools used and the pipeline's current configuration there are two main issues: a lack of channel support for major versions, and an inability to host multiple minor versions for each major version. By fixing these issues we can allow customers to upgrade the teleport package to the latest minor version release for their major version, as well as allow them to roll back to a previous minor version if need be.

## Details

### Recommendations
Due to issues with the third party solutions, along with the additional channel support features available, I recommend fixing our current S3-hosted solution. This would require switching reprepro for aptly in our Drone pipelines, and updating the current createrepo steps.

GCP Artifact Registry is in preview and has not had an update since 11/2021. It's lacking some features as outlined above as well as some others that we may wish to use in the future (i.e. bringing our own key). In addition, if there was a disaster recovery event where we needed to push to an Artifact Registry in another region while GCP had an outage, we would break `apt update` and `yum update` for our customers as the key they would use is region-specific.

As discussed below, JFrog Artifactory and PackageCloud are non-starters due to their signing key requirements.

#### Implementation details and proof of concept
The following channel scheme is proposed for APT and YUM with the S3-hosted option:

APT: `deb https://apt.<domain>/<os> <os version> non-free/<stable/testing/nightly>/<v6/v7/v8/...>`

YUM: `https://yum.<domain>/<os>/<os version>/Teleport/<arch>/os/<stable/testing/nightly>/<v6/v7/v8/...>/`

These schemes follow the repo structure used by Debian, Ubuntu, RHEL, CentOS, and Rocky Linux.

A tool implementing the required APT changes is available [here](./build.assets/tooling/cmd/build-apt-repos/). Broadly speaking, the YUM changes can be completed by uploading the files created by `createrepo` to the `/<os>/<os version>/Teleport/<arch>/os/<stable/testing/nightly>/<v6/v7/v8/...>/` subpaths in the S3 bucket. In addition to the previous changes outlined, two buckets for the new APT and YUM will need to be created in the AWS `teleport-prod` account.

### Future work
While a specific solution is outside of the scope of this RFD, it is pertinent to discuss a disaster scenario that is common to all solutions, including the current one. If the hosting solution that contains the repo (i.e. a S3 bucket or GCP Artifact Registry) is deleted then all artifacts must be rebuilt and published from scratch. It looks like the Drone pipeline for Teleport takes around 90 minutes to run. Depending on how many instances can be ran at once without conflicting with each other, it could take several hours to get the repository back online to it's previous state. This could be alleviated by backing up artifacts after they're built, or by backing up the entire hosting solution. 

#### Backwards compatibility
To maintain backwards compatibility with our current solution we will host both the new and old repos in parallel. We will also remove the old repo from Teleport's documentation, replacing it with the new repo. This will prevent our customers from seeing a breaking change while migrating new users to the new repo.

### Research
The current solution consists of using `reprepro` to build a new APT repo with only that latest Teleport version. There is no channel support and while all previous releases of Teleport are stored in the APT S3 bucket, `reprepro` only lists the most recent release in the index. YUM repositories are created with the `createrepo` tool. This tool has the flexibility we require to support multiple channels, but the pipelines are not currently configured to do so.

Fixing the current solution without moving to a third-part hosting solution is broadly defined as replacing `reprepro` with `aptly`, and by updating the APT and RPM publishing tool's configuration in the Drone pipeline to support channels.

Several potential solutions were investigated and their features compared as shown below:

| Feature/Product                            | Current solution           | Current solution with fixes                                                                                  | JFrog Artifactory                                                       | PackageCloud                                   | GCP Artifact Registry                                                             |
|--------------------------------------------|----------------------------|--------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|------------------------------------------------|-----------------------------------------------------------------------------------|
| Repo signing key goes to third party infra | Yes, Google (via Drone)    | Yes, Google (via Drone)                                                                                      | Yes (for SaaS)                                                          | Yes (for SaaS)                                 | Yes                                                                               |
| Who provides signing key                   | Teleport                   | Teleport                                                                                                     | Teleport                                                                | Either                                         | Google                                                                            |
| Built in third party secret provider support   | AWS, Hashicorp, Kubernetes | AWS, Hashicorp, Kubernetes                                                                                   | Hashicorp                                                               | No                                             | N/A                                                                               |
| APT channel support via components         | No ('main' only)           | Yes                                                                                                          | Yes                                                                     | No ('main' only)                               | No ('main' only)                                                                  |
| APT channel support via distribution       | No ('stable' only)         | Yes                                                                                                          | Yes                                                                     | No (Specific OS versions only)                 | Yes (via separate repositories)                                                   |
| APT channel support via URI                | No (one URI only)          | Yes                                                                                                          | Yes                                                                     | Yes                                            | Yes                                                                               |
| YUM channel support via URI                        | No (one URI only)          | Yes                                                                                                          | Yes                                                                     | Yes                                            | Yes                                                                               |                                                                       |
| Channel support notes                      | No support currently       | Can do anything with some reconfiguration                                                                    | Can do anything we care about with some reconfiguration                 | Very limited, no good solution                 | Missing some core features, would require CDN to rewrite HTTP header for requests |
| Monitoring and alerting support            | In house, poll based only  | In house, poll based only                                                                                    | Webhooks on important events                                            | In house, poll based only                      | In house, poll based only (only supported in AR for Docker images)                |
| Supports self hosting                      | N/A                        | N/A                                                                                                          | Yes                                                                     | Yes                                            | No                                                                                |
| Has official Terraform provider            | N/A                        | N/A                                                                                                          | Yes                                                                     | No                                             | Yes                                                                               |
| Pricing ($)                                | N/A                        | N/A                                                                                                          | $700/month                                                              | $700/month                                     | $0.1/GB/month stored, $0.09/GB/month egress                                       |
| Notes:                                     |                            | Can do anything we want, just depends on the amount of initial and recurring engineering effort is required | Higher complexity, but supports pretty much any use case we'd ever need | Easy to use but probably not the best solution | Still in preview, not generally available                                         |

### Signing key management
JFrog Artifactory and PackageCloud require handing over our repo signing keys to them, which is a non-starter. While GCP requires using their own signing key (which is used for all GCP repositories in a given region), it is assumed that their security is sufficient to protect said key. Lastly, fixing the current solution will keep us in control of the key, but will require us to continuing storing and securing it ourselves.

### Channel support
#### APT
The current solution does have support for APT. GCP Artifact Registry supports APT channels only via the "distribution" parameter of an APT source. This is also directly tied to the registry name inside of GCP. In addition, the URL for the repository depends on the GCP project ID which could cause issues for all clients if the GCP project was recreated or we wanted to change which project owned the repository.

Fixing the current solution would allow for channel support via standard practices for APT using APT distribution and component parameters. This is inline with what most APT repositories do, and supports the scheme that teleport users are most familiar with.

#### YUM
As with APT, the current solution does not support YUM channels. GCP would add support, but only by means of the project ID and repository name. As with APT this is a somewhat fragile and inflexible solution. Fixing the current solution would allow for channel support in whatever manner we like, allowing it to conform to best practices and standard naming scheme.
