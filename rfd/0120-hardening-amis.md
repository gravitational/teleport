---
authors: Trent Clarke (trent@goteleport.com)
state: Draft
---

# RFD 0120 - Delivering hardened AMIs

# Required Approvers
* Engineering: `@r0mant`
* Security: `@reedloden && @jentfoo`
* Product: `@xinding33 || @klizhentas`

## What

This RFD discusses structures and processes to increase the security of the
Amazon EC2 Disk Images (a.k.a AMIs) used to ship Teleport to customers.

## Why

One way our customers consume Teleport is by using a pre-built and  -configured
AMI to run Teleport in AWS EC2 instances. As a provider of these images we are 
(at least partially) responsible for the security of all software included in
the image, not just Teleport. 

We should endeavor to not ship vulnerabilities to our clients, even if those
vulnerabilities are not directly in our software.

This is not simply an academic exercise. Customers have been asking for hardened
AMIs - especially for AMIs that conform to well-known security benchmarks. See 
[here](https://github.com/gravitational/teleport/issues/8078) for an example.

## Details

### Current Situation

We currently use [Packer](https://www.hashicorp.com/products/packer) and a shell
script to provision our AMIs, based on Amazon Linux 2. The same Packerfile is used
as a basis for several AMIs

Inclusions for all AMIs:
  * [Telegraf](https://www.influxdata.com/time-series-platform/telegraf/)
    for capturing monitoring data
  * [InfluxDB](https://www.influxdata.com/) as a data store for Telegraf
  * [Grafana](https://grafana.com/) to display time series data
  * [nginx](https://nginx.com) to serve Grafana's front end  
  * EFF [`certbot`](https://https://certbot.eff.org/) for ACME Certificate
    Rotation (used by optional ssm `systemd` units)
  * Python 3 and `pip` for installing and running `certbot`
  * `uuid` for token generation (used by optional ssm `systemd` units)
  * `gcc` for ???
  * `libffi-dev` for ???
  * `openssl-devel` for ???
  * `libfontconfig` for ???

Whether or not the actual services are *enabled* depends on post-build 
actions by the eventual AMI consumer, such as enabling various `systemd`
units.

### Current Benchmarking

We currently do not test our images against a benchmark.

### Proposed Solution

 * Build a Teleport maintained STIG-high AL2 "Golden" base image using the
   AWS EC2 Image Builder
 * Use this hardened image as the base for all Teleport supplied AMIs built
   with Packer
 * [Optional] Slimming the AMIs to reduce attack surface
 * Post-build validation using Amazon Inspector
 * Ongoing vulnerability scanning with Trivy

#### **CIS vs STIG**

The two best-known standards for security hardening are 
 * CIS Benchmarks (Level 1 & 2), and
 * The US DoD [Security Technical Implementation Guides](https://public.cyber.mil/stigs) (STIGs)

Both of these standards are pretty comprehensive and have wide
application, although private organizations tend to favor CIS
over STIG. 

So why choose STIG over CIS?

The decision largely comes down to what tooling is immediately available. Both
standards are comprehensive - the AL2 CIS benchmark document runs to about 600 
pages. At an estimated 2 pages per item, that's 300 separate items that need
detection, remediation and validation. I don't believe that Teleport wants 
to maintain scripts to do that ourselves, so I have been focused on finding 
tools to do it for us.

Somewhat unsurprisingly, nearly all of the automated AMI hardening tools I have 
found are tied to the AWS EC2 Image Builder Tool. Both standards are represented, 
but while the STIG-compliance components are free for use on any image, the 
CIS tooling has several drawbacks:

  1. The hardening components require the use of a CIS-supplied base image that
     incurs an added per-minute royalty to CIS. As we are using this image as a
     delivery channel to our customers, this is a non-starter.
  2. The use of the CIS hardening tools requires a subscription to/partnership with 
     CIS that we do not currently have

The decision to go with STIG is based on being able to improve the situation
for our customers *now*, while still being able to add CIS hardening tools as
we develop that relationship in the future.

#### **How do security benchmarks relate to AMIs?**

Both the CIS Benchmarks and STIG e es

#### **Hardened Base Image**

My proposed solution is to use AWS EC2 Image builder, using the available STIG
tooling, to create a hardened base image. 

We can do this as part of the build process, or on some frequent schedule 
(e.g. daily, weekly).

I suggest the latter, scheduled option;

 1. The release build process is already complicated enough, and
 2. There is no real advantage to creating a per-release base image, as there 
    is no way to inject parameters into the image build pipeline.

#### **Why ImageBuilder *AND* Packer?**

As mentioned above, there is no way to inject parameters into an EC2 ImageBuilder
pipeline. The pipeline is the pipeline. There is some customization available at
the build component level, but there is no way (that I can see, at least) to
say something like "Trigger this pipeline using Teleport `vX.Y.Z` and save the
resulting image as `teleport-oss-X.Y.Z`". All you can do is trigger the pipeline
and have it produce (or replace) a single image.

So this brings us to an obvious division of labor:

 * **EC2 ImageBuilder** to build the hardened base image, which will be used as
   the common base for all AMIs we produce, across all supported Teleport
   versions, and
 * **Packer** for the final, individual AMIs that are tailored to a specific
   Teleport version and configuration.

There is a side benefit here: were we to move the entire process into EC2 Image
Builder, it would require moving a lot of Teleport-specific code and configuration
*out* of the `teleport` Git repositories and into `teleport-cloud` terraform.

From experience, keeping Teleport-specific things closer to `teleport` is generally
a better idea than splitting them apart.

#### **[Optional] Slimming the AMIs**

In the spirit of moving towards distroless OCI container images, I propose slimming
the contents of the AMIs in order to reduce their attack surface. 

#### **Publishing to `teleport-prod`**

These new images will be published to the `teleport-prod` AWS account. This will
partially obviate the need to move the legacy images from the `gravitational`
AWS account. 

As per the existing AMI builds, AMIs will be constructed as part of the tag build, and
made public on promotion.

#### **Post-build validation using Amazon Inspector**

While the CIS-hardening tooling on Amazon requires royalties and subscriptions,
the Amazon Inspector tool contains CIS benchmarks appears to be royalty free. 

Amazon Inspector can't examine AMIs directly, but it is possible to 
 1. spin up an EC2 instance based on the given AMI, 
 2. install the AWS Inspector agent onto the Instance,
 3. run an Inspector assessment over the instance,
 4. interrogate the resulting list of findings to decide if a build passes
    or not

This idea is based on a [Continuous Vulnerability Assessment](https://aws.amazon.com/blogs/security/how-to-set-up-continuous-golden-ami-vulnerability-assessments-with-amazon-inspector/) example from the AWS security blog, minus the scheduled lambda to trigger the assessment.

#### **Ongoing Scanning**

We should mirror our approach for OCI container Images ([See RFD-0112](./0112-securing-docker-images.md)), which boils down to:

 * Routine (e.g. daily) scanning of the most recent release of each 
   supported Teleport version (i.e. 3 most recent major versions). 
 * Detected issues written to the `teleport.e` GitHub Security Issues 
   tab
 * Any further processing to be handled by our Panther SIEM

The `trivy` scanner is [known to support scanning AMIs](https://aquasecurity.github.io/trivy/v0.35/docs/vm/aws/), 
and we already it for OCI container images and other resources. We should
use it for our AMIs as well.

#### **AMI Cleanup**

There is a default 1114 limit of public AMIs allowed in an AWS region. While this 
can be raised on request, it seems reasonable to periodically clean up our public
images.

Proposed deletion criteria:
 * non-release builds (i.e. anything with a semver suffix) older than 6 months
 * release builds with Teleport major version <= 5 (updated over time )

### **Implementation Plan**

  * **Phase 1:** 
    * Hardened base image, 
    * re-based AMIs, 
    * publishing to `teleport-prod`
  * **Phase 2:**
    * Post-build validation
  * **Phase 3:**
    * Ongoing scanning
  * **Phase 4:**
    * AMI Cleanup

I expect that Phases 2-4 can be performed in any order, even in parallel,
depending on what is the highest value target.

### **Migration Plan**

Changing the contents and configuration of the shipped AMIs is considered a
compatibility breaking change, and so needs to be handled with some care. 

We will be rolling out the images over 3 major releases of Teleport:

1. **Teleport 13**: begin publishing hardened images to `teleport-prod` in
   parallel with legacy images in `gravitational` account.
2. **Teleport 14**: make hardened images the default, and deprecate (but 
   still publish) legacy ones
3. **Teleport 15**: stop publishing legacy images

