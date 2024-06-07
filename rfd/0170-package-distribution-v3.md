---
authors: Fred Heinecke (fred.heinecke@goteleport.com)
state: draft
---

# RFD 170 - Package Distribution v3

## Required Approvers

* Engineering: @r0mant
* Product: @klizhentas

## What

Hosting and configuration of APT, YUM, and Zypper repositories, and publishing
to them. Other binary artifact repos (release server, Helm, Terraform), are
considered.

## Why

RFD 58 defined our current OS package repo solution. The solution built based
off of this does not meet our current needs, which were primarily missed
because:
* When I wrote the RFD I was new to Teleport, and did not fully understand the
  impact of the work,
* The requirements for this project were not fully captured,
* We did not fully anticipate how moving to GHA would affect the specified
  solution.

This has lead to the following problems:
* OS package publishing takes a very long time. It dwarfs the publishing time of
  all other publishing steps. This is expensive, and makes any changes to the
  publishing process difficult to test.
* It is vulnerable to a cache poisoning attack. The current solution relies on
  massive self-hosted caches in our EKS clusters. If an attacker is able to gain
  access to the cache, then they could exploit this to overwrite published
  artifacts. This could be used by an attacker to take over both internal
  systems, and customer systems.
* The solution is extremely technically complex, and very brittle. Any change
  remotely related to OS package publishing requires extensive manual testing,
  and even with this, we still periodically break customer systems due to how
  fragile the current solution is.
* We are unable to make certain changes to how we publish artifacts, resulting
  in lots of lost development time on certain projects (such as auto upgrades)
  working around these limitations. For example, we cannot set _any_ APT repo
  options to tell customer APT clients how to properly use our repos.
* We cannot pull packages[^1]. The need to do this is infrequent, but the impact
  on both our internal teams and customers is very large.
* Disaster recovery events take days to complete. Internally this often halts
  work, and externally it can mean outages for customers. During disaster
  recovery, we cannot publish releases, including auto update fixes.
* Both maintenance and hosting of the current solution is expensive, as well as
  publishing to it. There are regular issues with it that require devs to drop
  everything to keep the solution online and working. Hosting the solution costs
  in the ballpark of $10k/year in AWS infra costs alone, not counting data
  transfer costs to clients. The cost to publish a release to OS package repos
  is estimated at $5 per release on average. At 3 releases per day (estimated,
  average, including dev tags), 6 days a week, 52 weeks a year, this comes out
  to another $5k/year. Dev time is much harder to estimate. Assuming a burden
  rate of $175/hour, and an average total of 10 hours per week wasted by devs
  due to the current solution (primarily maintenance + waiting for job
  completion), this solution is costing $91k/year in dev time.
  
  AWS shows that we're storing roughly 1.25 TB of artifacts in APT and YUM S3
  buckets. It also shows that customers download about 180 TB of artifacts every
  month, via Cloudfront. This is increasing by approximately $800 month. The
  total cost to us for downloads last month was $11.9k, and the total cost for
  last year was approximately $75k. Assuming the last year's cost increase trend
  continues, this coming year will cost us about $200k.

  The total cost of this solution over the next year will _conservatively_ be
  almost $300k. This does not take into account the (hard to quantify)
  opportunity cost of dev time spent operating and managing the current
  solution, which could be spent on higher-value work.

One thing RFD 58 did get right is the structure of the OS package repos. This
does not need to change, and the RFD will not discuss any changes to this.

## Details

Any new solution must meet the following:
* Customers should not need to know about the new solution, or be negatively
  impacted by it. This means no loss of availability, no regression in
  functionality, and no required change for how customers install our products.
  The implementation of a new solution must be _entirely_ transparent to
  customers.
* We must not be required to provide sensitive secrets to any new third-party
  vendor, including signing keys for the repos metadata. These keys must never
  leave the trusted systems that we control.
* The new solution must meet all of our typical security requirements for
  release pipelines (audit logging, RBAC or Teleport support, etc.) and must
  pass an external security audit.
* The solution must be cost effective, with a TCO less than estimates of our
  current solution.
* The total time to publish all the OS packages per release must be under ten
  minutes, in an "apples to apples" comparison (same number of packages in
  repos, same number of new packages, same number of repos, etc.)
* The new solution must be technically simple, compared to our current solution.
  Our current solution is tightly coupled with multiple repos, auto updates
  tooling, GHA itself, our GHA EKS cluster, and several internal tools. It
  should be "easy" for us to:
  * Add and remove repos
  * Add and remove packages/products
  * Configure repo-type-specific properties (such as APT
    Phased-Update-Percentage)
* The customer-facing portion of the solution (the repos themselves) must be
  highly available and fault-tolerant. No regular maintenance tasks (such as
  upgrades of the solution) may result in downtime or outages of the
  customer-facing portion.

The following would be nice to have, but not required:
* Support for other binary artifact types. This could help standardize how we
  publish artifacts, and reduce some of the maintenance costs associated with
  other in-house tools that we've built over the years.
* Download metrics. This would help us in several ways, from gauging when we can
  deprecate certain channels, to the impact of a customer-facing issue.
* Management via IaC. Ideally we should be able to deploy new repositories and
  manage permissions in a way that meshes with how we manage these elsewhere in
  the org.

### Potential solutions

I have evaluated several potential solutions. Some of these were evaluated as a
part of RFD 58, but have undergone significant changes and have been
re-evaluated.

#### Home-grown solutions

Our current solution is built in-house, relying primarily on two third party
tools (Aptly and createrepo_c) for the generation of repo metadata. I've
followed this space since the implementation of RFD 58 and there have not been
significant changes in the past couple of years. There are two routes we could
take if we wanted to continue using a system that we build and maintain:

1. Scratch-built web service for repo metadata generation and package
   publishing, and S3 + CloudFront for providing the repos to customers.
2. Build tooling on top of [Pulp](https://pulpproject.org/), a FOSS tool to
   manage package repos.

##### Scratch-built web service

This could be built entirely from scratch, or built into the release server.
This route would allow us to customize the tooling to meet our needs exactly.
However, there are not currently any Golang libraries that provide tooling to
create and manage APT or YUM repos[^2]. This means that we would need to build
and maintain this code ourselves, or use another language. Writing and
maintaining these libraries has been evaluated several times over the past
couple of years, and decided against as the development and maintenance costs
would be high.

I looked for deb and rpm repo libraries for JS/TS, Python, Rust, C#/.NET, and
Java. Of all these languages, the only ones that I found both [deb and rpm
repository libraries](https://pypi.org/project/mkrepo/) for was Python. This
could _potentially_ be used with Temporal to build a package deployment service,
but it has some major drawbacks:
* The Python library is not regularly maintained (last release was about 18
  months ago), and we'be be relying on a small handful of third-party developers
  for one of the backbone of our OS repositories.
* The Python library is not written to be used as a library, so we would need to
  either fork and convert it, or invoke it as a binary/language-independent
  tool, similar to what we're doing today with Aptly.
* The tool has some bugs and missing features that are critical to our technical
  requirements.
* We would still need to manage everything outside of this tool itself (such as
  mutex locking), which we've been bitten by pretty regularly in the past.
* The tooling for this would be built around yet another language, complicating
  the maintenance of our release process further.

Ultimately we would _need_ to fork this tool and put in a bunch of work just to
get it into a state where we could use it, and then we'd need to continue to
maintain it long-term. I do not believe that doing this would save us
significant effort over implementing the entire deb and rpm repo specs from
scratch in Go.

##### Pulp-based tooling

[Pulp](https://pulpproject.org/) would allow us to focus less on the repo
generation logic, and more on how the tool integrates with our business needs.
It consists of several services that we would need to host (front end, API
service, plugin service, database, cache, storage, CDN). It is actively
maintained by Red Hat.

However, it has several downsides:
* It's written in Python, so any changes to the (collective) Pulp service would
  likely also need to be written/maintained with Python.
* It's plugin based, so each repo type (APT, YUM, etc.) has an entirely
  different interface from the ground up. Despite being one collective service,
  the requirements, capabilities, and APIs for each are wildly different. For
  example, the YUM plugin supports a S3 backend while the APT plugin does not.
* The documentation is severely lacking, and hard to use. There is no paid
  support for this tool.

It is my strong opinion that this would be a maintenance and security nightmare.

#### Third party solutions

There are many options for third party solutions to our problem:

* Artifactory
* PackageCloud (requires providing signing key)
* Nexus Repository
* Gitea
* GitLab
* ProGet
* AWS CodeArtifact (does not support apt/yum)
* Azure Artifacts (does not support apt/yum)
* GCP Artifact Registry (requires providing signing key)
* Cloudsmith (requires providing signing key)

I've taken a look at all of them and demoed a few. Here's what I found.

##### AWS CodeArtifact and Azure Artifacts

Both AWS and Azure offer package hosting solutions. An AWS service would be
great, as we already trust AWS with many of our sensitive secrets.
Unfortunately, neither of these products support APT or YUM repos.

##### PackageCloud, Cloudsmith, and GCP Artifact Registry

All three of these would be potentially viable options for OS package hosting,
except they violate one of the core requirements for a replacement: they all
require handing over our signing key.

GCP _might_ be the only case where we are willing to make an exception. We would
still need to provide them our signing key, but given that they are one of the
"big three" cloud providers, they are arguably more trustworthy than the other
two SaaS vendors.

###### GCP Artifact Registry TCO
My estimate of the GCP infra cost, based off of our current repo usage, is
$167k/year. This is broken down into:
* $3k/year in storage costs.
* $164k/year in data transfer costs.

The cost to properly secure and integrate with GHA for one of our most sensitive
processes is much harder to calculate. Unfortunately I don't have remotely close
to enough information to even guess at this, so the best I can estimate for the
TCO is that it's >= $167k/year.

##### Gitea and GitLab

Gitea and GitLab are traditionally used as Git version control platforms.
However, they both now have support for package registries. They can also both
be self hosted, which negates the signing key possession issue.

While GitLab does have experimental support for APT repos, it does not have any
support for YUM repos. This is a non-starter, so GitLab does not fit our
requirements.

Gitea does support both APT and YUM repositories, as well as a host of other
package repository types. The downside is that all requests for the APT and YUM
repos must go through a self-hosted Gitea web service. Unlike our current
S3/CloudFront based solution, if we wanted to guarantee of uptime, then this
would likely require a 24/7/365 on-call schedule for the team that would manage
it.

##### ProGet

ProGet is a paid, self-hosted tool for hosting package
repositories. It's primary focus is delivering packages for the Microsoft
ecosystem for Microsoft shops, but it does have support for APT and YUM repos.
However, there are several downsides to this product:

* The licensing cost alone would be $25k-$50k per year, which is in the realm of
  other possible solutions. However, other competing solutions at a similar
  price point are more polished, and appear to support our needs better (more on
  this in their respective sections).
* As with Gitea, this approach would require all customer requests for the repos
  to flow through the ProGet web service. This would likely require a 24/7/365
  on call schedule for the team that manages it.
* The container image for this solution is massive, and contains an entire Linux
  distribution inside. This results in a large attack area that we would be
  responsible for securing, for one of our most sensitive services.
* The company has been around for almost 20 years. However, in that time, they
  do not appear to have grown significantly, or innovated much. They also don't
  appear to have many customers, and they don't appear to have customers with
  needs on the same scale as us. I am not confident that they have the size or
  speed needed to support us.
* To company's About page, their tools are "Windows first, not as an
  afterthought." They go on to say, "Instead, we prefer Windows, and always
  start development, testing, and documentation in Windows." While this may be a
  selling point for some of their customers, we are a Linux and macOS driven
  shop. I suspect that the ProGet will not be able to support us as well as if
  we were one of their target customers. This is corroborated in [one of their
  docs
  pages](https://docs.inedo.com/docs/proget-feeds-other-types#:~:text=For%20example%2C%20when,use%20these%20languages).

##### Artifactory and Nexus Repository

Both Artifactory and Nexus Repository are similar in their design, features, and
price. Here's where they are similar:
* Both support APT and YUM repos, as well as a host of other package repo types.
  We could potentially replace several other package repositories with one of
  these solutions. Doing so would reduce both our tooling complexity, dev time,
  and costs. Here's a list of other artifact types that we support today that we
  could shift to one of these solutions:
  * Helm
  * Terraform (Nexus Repository requires a third-party plugin)
  * Release server/raw binary artifacts

  They both also support pull-through caches and/or mirroring for Go modules,
  Rust crates, NPM packages, and container images. With their built-in security
  and license compliance tool, eventually using one of these solutions as a
  proxy for upstream repos could go a long way towards improving our
  supply-chain security.
* Both would need to be self hosted. This negates the signing key possession
  issue.
* Artifactory supports the S3/CloudFront architecture that we currently have,
  meaning that the team that manages the service does not need to be on call
  24/7/365. Additionally, this does not mean that there would be a new cost
  overhead associated with every download. I'm waiting to hear back from
  Sonatype to confirm if Nexus Repository supports the S3/CloudFront
  architecture as well.
* Neither solution would require any customer-facing change. Our repos would
  continue being structured as they are now.
* Both appear to support our general security requirements, with IdP support,
  fine-grained RBAC, and audit logs. Artifactory even supports passwordless
  authentication with GHA, similar to GHA AWS authentication.
* Artifactory and Nexus Repository can be deployed via Helm charts with fairly
  minimal effort. The system architecture for each is a n-tier webapp, with a
  handful of smaller services. They both rely on an external database, as well
  as blob storage. We would probably need to implement this as a RDS cluster +
  S3 bucket(s).

  While this is more complex than some of the other solutions, this is still
  notably less complex than our current system.
* Both can be configured via IaC/Terraform. This allows us to review changes
  before they're deployed, and allows us to keep a historical record of what
  changed, when, and why.

  Note that Nexus Repostory's Terraform provider is managed by a third party,
  and does not have first-class support from Sonatype.
* Both support some degree of download statistics. However, support for download
  statistics on each is not great. Artifactory statistics are only available via
  API, and Nexus Repository does not support statistics in a HA environment. I
  expect that both would have additional issues issues with generating accurate
  statistics, when they are fronted by a CDN that routes to S3 rather than their
  web service.

The license pricing is one place that they both differ. **Assuming** that the
pricing information for self-hosted Nexus Repository is similar to their SaaS
offering, licenses for Nexus Repository would cost around $24k/year, and would
scale with the number of employees we have.

###### Nexus Repository TCO
My estimate of the AWS infra cost, based on [this
architecture](https://help.sonatype.com/en/nexus-repository-reference-architecture-5.html),
is $280k/year. This is broken down into:
* $30k/year via 5x `m5a.4xlarge` EC2 instances.
* $40k/year via 3x `m4.2xlarge` EC2 instances in an RDS PostgreSQL cluster, as
  well as other misc. related charges. Note that I have very little to go on
  here - the docs are unclear on the database performance requirements. This may
  be wildly inaccurate.
* $200k/year on S3 + CloudFront transfer costs. This assumes that we won't need
  to store any additional data, and carries all the same assumptions about
  traffic pattern that I've made for our current solution.
* $10k/year on misc. other charges, such as internal network costs, increased
  monitoring costs, KMS costs, and anything else that I missed.

My estimate of employee time, under the same burden rate assumed for estimates
of our current solution, is $5k/year. This is broken down into:
* $5k/year for 30 minutes per week spent managing this solution once deployed
* $0k/year spent waiting on OS package publishing pipelines, as OS package
  publishing will no longer be the longest running job.

This comes out to about $306k/year. **It is important to note** that if we can
route all traffic through S3/CloudFront, then the load will be significantly
lower, which means that we may be able to reduce the AWS costs from $280k/year
to $20k-$50k/year.

###### Artifactory TCO
Artifactory licensing pricing is higher. Licensing fees would cost us $3.8k per
month ($45.9k/year), or more if we end up needing their "Enterprise+" plan.
However, this would not increase with employee count.

My estimate of the AWS infra cost, based off of various places in their docs, is
$217k/year. This is broken down into:
* $2.7k/year via 1x `m6g.2xlarge` EC2 instances.
* $40k/year via 3x `m4.2xlarge` EC2 instances in an RDS PostgreSQL cluster, as
  well as other misc. related charges. Note that I have very little to go on
  here. This may be wildly inaccurate.
* $200k/year on S3 + CloudFront transfer costs. This assumes that we won't need
  to store any additional data, and carries all the same assumptions about
  traffic pattern that I've made for our current solution.
* $10k/year on misc. other charges, such as internal network costs, increased
  monitoring costs, KMS costs, and anything else that I missed.

My estimate of employee time is the same as Nexus Repository.

This comes out to about $268k/year. As with Nexus Repository, our actual cost
will likely be lower (I'd estimate about $30k/year lower) if we can route all
traffic through S3/CloudFront.

###### Other impacts on TCO
Artifactory comes bundled with a large number of additional optional tools.
Of note, these include:
* A complete CI/CD pipeline service (competitor to GHA)
* Supply chain security tools:
  * Package vulnerability scanning
  * SBOM production from multiple sources
  * Secrets scanning
  * Multi-source vulnerability database
  * PR-time IaC scanning (we can't do this today)
  * Automated risk estimation of third party dependencies (we can't do this today)
  * Single pane of glass overview of entire org

In my opinion, the benefit of some of their security tools is enormous and would
go a long way towards satisfying some of our security-related OKRs.
Additionally, these tools could replace several separate third-party tools,
which would reduce our attack surface and potentially reduce the number of
vendors we contract with.

### Comparison of viable solutions

Out of the twelve solutions investigated, only the following can meet our
requirements:
* Scratch-built in-house solution
* Gitea
* GCP Artifact Registry
* Nexus Repository
* Artifactory

Here's a table showing their tradeoffs:
| Name                  | TCO                     | Requires on-call rotation | Requires giving away signing key | Supports other high-level goals |
| --------------------- | ----------------------- | ------------------------- | -------------------------------- | ------------------------------- |
| Current solution      | $300k/year              | No                        | No                               | No                              |
| In-house solution     | Unclear - likely high   | Unlikely                  | No                               | No                              |
| Gitea                 | Unclear - likely high   | Yes                       | No                               | No                              |
| GCP Artifact Registry | >= $167k/year           | No                        | Yes                              | No                              |
| Nexus Repository      | $256k/year - $356k/year | Unclear                   | No                               | Yes                             |
| Artifactory           | $238k/year - $268k/year | No                        | No                               | Yes                             |

One thing that has not been addressed yet in this RFD is the package deployment
time for each solution. I will not be able to determine this until I demo any
solutions that we are interested in. My recommendation is that this RFD goes
through a first round of review, and then I'll demo any desired solutions and
get an estimate of the deployment time. I'll then update the above table with
this information, and remove this section.

### Recommendation
My recommendation is that we explore replacing our current solution with
Artifactory. This is the lowest cost solution with the fewest drawbacks and most
advantages.

Here are the next steps if we choose to go this route (after testing other
competing solutions that we are interested in):
1. Contact JFrog and negotiate with them for a PoV deployment.
2. Attempt to replace our staging OS package repo tooling and infra with
   Artifactory.
3. Load our production packages (several TB) and load test.
4. Use Artifactory as a replacement for our current solution in staging for a
   month or two.
5. If everything goes smoothly, go through the vendor onboarding process with
   them and sign a contract.
6. Deploy Artifactory to production and migrate to it.

### Future work
If we decide to go with Artifactory, then there are several pieces of future
work that we should take on to fully realize it's benefits:
* Setup mirrors and/or pull-through caches for build time dependencies (i.e. Go
  modules, third-party OS packages, NPM packages, etc.). This would increase the
  stability and speed of our build process, and potentially reduce costs due to
  the decrease in build time.
* Replace some of our security tools with the security product that comes
  bundled with Artifactory (Xray). This would reduce the number of third-party
  dependencies we have, improve security, and potentially eliminate some other
  vendors.
* (Long term) Evaluate the CI/CD product that comes bundled with Artifactory
  (Pipelines) as a potential replacement for GHA. I currently have no knowledge
  of this product or if it's a viable alternative, but I think that it is worth
  evaluating.

[^1]: _Technically we can, but it takes enormous effort (entire team needs to
    drop everything to spend hours working on this) and is error prone_
[^2]: _There are some tools *built* with Go to do this, but none of them have a
    stable library interface, and/or are not-production ready for other
    reasons._