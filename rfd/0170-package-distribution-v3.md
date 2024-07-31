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
off of this does not meet our current needs. This has lead to the following
problems:
* OS package publishing takes a very long time. It dwarfs the publishing time of
  all other publishing steps, [typically taking about 30m, with the next closes
  publishing step taking about
  10m](https://github.com/gravitational/teleport.e/actions/runs/10118540375).
  This is expensive, and makes any changes to the publishing process difficult
  to test.
* It is vulnerable to a cache poisoning attack. The current solution relies on
  massive self-hosted caches in our EKS clusters, which are available and used
  by the OS package repo tool GHA runners. If an attacker is able to gain access
  to the cache, then they could exploit thi to overwrite published
  artifacts. This could be used by an attacker to take over both internal
  systems, and customer systems.
* The solution is extremely technically complex, and very brittle. Any change
  remotely related to OS package publishing requires extensive manual testing,
  and even with this, we still periodically break customer systems due to how
  fragile the current solution is. Here are some specific examples of
  issues caused by it in the past three months alone:
  * Dev tag pushed to prod and customers began updating to it. This was
    discovered late in the afternoon on Thursday and if I hadn't happen to be
    working late, a lot more customers would have been impacted by it. It took
    two engineers about six hours each to pull the package. More details
    available upon request (in private Slack channel).
  * Package violating a key security constraint found during the above incident
    response. Package had been published for months and we only happen to notice
    when querying an internal SQLite database that one of the system's
    dependencies uses.More details available upon request (in private Slack
    channel).
  * [Unable to pull packages with less than twelve engineer hours worth of work,
    causing a major outage for a customer while we were trying to close the
    largest single deal in company
    history](https://gravitational.slack.com/archives/C04P78M46F2/p1718125441924689).
  * [A couple of relatively simple changes to the system's pipelines caused
    release times to increase from thirty minutes to three to six
    hours](https://gravitational.slack.com/archives/C04P78M46F2/p1717726105917779).
    This caused major issues trying to get `fdpass` out the door for a large
    prospective customer.
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
* Customer-facing requests (such as those generated when a customer runs `apt
  install`) must not run through a service that we are responsible for keeping
  online.
* The customer-facing portion of the solution (the repos themselves) must be
  highly available and fault-tolerant. No regular maintenance tasks (such as
  upgrades of the solution) may result in downtime or outages of the
  customer-facing portion.
* We must not be required to provide sensitive secrets to any new third-party
  vendor, except for one of the "big three" Cloud providers. This includes
  signing keys for the repos metadata. These keys must never leave the systems
  and services that we trust.
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
* We cannot go with any SaaS solution except one provided by a major Cloud
  provider (AWS, GCP, Azure) that we already trust.
* The solution must be manageable via some form of "GitOps", where all changes
  are checked into a git repository after being peer reviewed. Ths likely means
  that the solution will need to include a Terraform provider for the product.

The following would be nice to have, but not required:
* Support for other binary artifact types. This could help standardize how we
  publish artifacts, and reduce some of the maintenance costs associated with
  other in-house tools that we've built over the years.
* Download metrics. This would help us in several ways, from gauging when we can
  deprecate certain channels, to the impact of a customer-facing issue.

### Potential solutions

I have exhaustively evaluated all potential solutions on the market today. Some
of these were evaluated as a part of RFD 58, but have undergone significant
changes and have been re-evaluated.

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
Java. Of all these languages, the only one that I found both [deb and rpm
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

I estimate that we would need a minimum of a full engineering-quarter worth of
work for either of these routes. Excluding the opportunity cost of not being
able to work on other projects, I estimate that the development costs of this
would be $100k worth of engineering at absolute minimum. The infra and
maintenance costs would likely be similar to our current in-house solution,
costing a minimum of $300k/year on top of the initial development costs.

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

There are many options for third party solutions (both SaaS and on-prem) to our
problem:

* AWS CodeArtifact
* Azure Artifacts
* Cloudsmith (requires providing signing key)
* PackageCloud (requires providing signing key)
* GCP Artifact Registry (requires providing signing key)
* GitLab
* Gitea
* ProGet
* Artifactory
* Nexus Repository

I've taken a look at all of them and demoed a few. Here's what I found.

##### AWS CodeArtifact and Azure Artifacts

Both AWS and Azure offer package hosting solutions. An AWS service would be
great, as we already trust AWS with many of our sensitive secrets.
Unfortunately, neither of these products support APT or YUM repos.

##### Cloudsmith and Packagecloud

All three of these would be potentially viable options for OS package hosting,
except they violate one of the core requirements for a replacement: they both
require handing over our signing key.

##### GCP Artifact Registry

GCP Artifact Registry is a SaaS solution that requires handing over our signing
key, however, GCP is a "big three" Cloud vendor.

However, GCP Artifact Registry fails several requirements:
* We would need to implement major customer-facing changes that would break
  current installs and the auto updater. This would include:
  * A complete restructuring of our repository layout and installation scheme
  * Switching to a GCP-branded domain name (i.e. `us-central1-yum.pkg.dev`
    instead of `yum.releases.teleport.dev`)
  * Potential namespace collisions with other third parties, as APT/YUM repos in
    GCP Artifact Registry are region-global, similar to AWS's S3 buckets
* The solution would be relatively complex in that we would need to onboard
  another major cloud vendor. One of the reasons we moved off of them several
  years ago is because the benefit we gained by using a couple of smaller
  services provided by GCP did not outweigh the security and compliance costs.

This solution was reviewed several years ago as a part of RFD 58, and since then
there have been no significant changes to the service. The docs also reference
long deprecated `apt` commands. Additionally, the docs are primarily focused
around usage as a private, internal company service. This implies that this
service is not a priority for GCP, not a good fit for our use case, and is
probably not something that we should even consider using even if we are willing
to accept the breaking changes.

##### Gitea and GitLab

Gitea and GitLab are traditionally used as Git version control platforms.
However, they both now have support for package registries. They can also both
be self hosted, which negates the signing key possession issue.

While GitLab does have experimental support for APT repos, it does not have any
support for YUM repos. This is a non-starter, so GitLab does not fit our
requirements.

Gitea does support both APT and YUM repositories, as well as a host of other
package repository types. The downside is that all requests for the APT and YUM
repos must go through a self-hosted Gitea web service. This fails our
"customer-facing requests must not run through a service that we are responsible
for keeping online" requirement.

##### ProGet

ProGet is a paid, self-hosted tool for hosting package repositories. It's
primary focus is delivering packages for the Microsoft ecosystem for Microsoft
shops, but it does have support for APT and YUM repos.

However, this fails our "customer-facing requests must not run through a service
that we are responsible for keeping online" requirement.

Additionally, there are several other downsides to this product:

* The licensing cost alone would be $25k-$50k per year, which is in the realm of
  other possible solutions. However, other competing solutions at a similar
  price point are more polished, and appear to support our needs better (more on
  this in their respective sections).
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

This is not a viable solution for us.

##### Nexus Repository

Sonatype Nexus Repository could be a great fit for us. It can be self hosted,
supports both APT and YUM repos without customer-facing changes, and can be
managed via GitOps (with a third-party Terraform provider). However,
customer-facing download requests would need to flow through a service that we
are responsible for keeping online and available, failing this requirement.

##### Artifactory

JFrog Artifactory meets all of our requirements, and is the *only* solution on
the market today that does so. In addition to meeting our requirements, we would
see several other benefits with this solution:

* Artifactory has support for several other repository types that we currently
  have. By (eventually) using Artifactory for these, we could get rid of a lot
  of tooling and infrastructure. Here's a list of repos types that we could
  shift to this solution:
  * Helm
    * Currently requires custom tooling for our release pipelines
  * Terraform (Nexus Repository requires a third-party plugin)
    * Currently requires extensive custom tooling for our release pipelines, as
      well as tooling for disaster recovery
  * Release server/raw binary artifacts
    * Currently requires a large amount of tooling in a monolithic service that
      struggles to support our current needs
* There is support for pull-through caches and/or mirroring for all major
  toolchains that we use in our product today. By using Artifactory (eventually)
  as a pull-through cache, we could scan all dependencies for vulnerabilities,
  malicious code, and changes in licensing prior to download. This would not
  only increase stability of our release pipelines, but also improve the
  security of our developer endpoints. here's a list of package types that are
  supported here:
  * OS packages (debs, rpms)
    * We don't pin these at all in our release pipelines, meaning we currently
      blindly trust the upstream sources
  * Go modules
    * We pin these currently but do not actively scan these for new/unreported
      vulnerabilities
  * Rust creates
    * We pin these currently but do not actively scan these for new/unreported
      vulnerabilities
  * NPM packages
    * We have constant failures due to issues with upstream sources
    * NPM has a history of being used for distributing malware, and JFrog's
      solution [has a history of detecting
      it](https://jfrog.com/blog/malware-civil-war-malicious-npm-packages-targeting-malware-authors/)
  * Container images
    * We don't often hit rate limiting here, but only because we pass around
      shared dockerhub credentials, which this solution could eliminate
    * We scan our built images for security issues, but not the images we pull
      in to build our releases (which have huge amounts of access to sensitive
      data)
* We could replace several of our current security tools **and vendors** with
  tools that come included with a self-hosted JFrog license. This aligns with
  our high-level objective of reducing our dependence on vendor services,
  despite adding a new vendor. This includes:
  * Orca (SaaS service)
  * FOSSA (SaaS service)
  * Trivy (OSS tool)
  * govulncheck (OSS tool)
  * actions/dependency-review-action (OSS tool)
  * Git pre-commit hook for secrets scanning (in-house tool)
* The entire security-reports repo could be replaced with JFrog's tools, at no
  additional cost.
* JFrog's solution has support for automated risk estimation of third party
  dependencies, which is something that we can't do today.

I don't have a quote from JFrog yet, but they have stated that the total cost of
ownership (including infra and maintenance) of their solution is in-line with
the cost of our current solution.

### Recommendation

Of the eleven evaluated solutions, only two meet our minimum requirements:
rewriting our in-house solution, or deploying JFrog's solution.

Given our long history of failing to build an adequate in-house solution (hence
the third iteration of this project), and the clear benefits that JFrog brings
to the table, I strongly believe that we should choose JFrog's offering. As
outlined above, JFrog's solution isn't just the only option that meets our
requirements for this project, but it also heavily aligns with other high-level
objectives.

### Next steps

Here are the next steps if we choose to go this route (after testing other
competing solutions that we are interested in):

1. Define a specific technical architecture for a production deployment with
   JFrog's help, and get an official quote from them.
2. Deploy a PoV deployment and load test, as a replacement for our staging OS
   package repo tooling and infra.
3. Load our production packages (several TB) and load test.
4. Use Artifactory as a replacement for our current solution in staging for a
   month or two.
5. If everything goes smoothly, go through the vendor onboarding process with
   them and sign a contract.
6. Deploy Artifactory to production and migrate to it.

### Future work
If we decide to go with Artifactory, then there are several pieces of future
work that we should take on to fully realize it's benefits:
* Move other package repos to Artifactory.
* Setup mirrors and/or pull-through caches for build time dependencies (i.e. Go
  modules, third-party OS packages, NPM packages, etc.).
* Replace some of our security tools with the security product that comes
  bundled with Artifactory (Xray).
* Replace or just remove the security-reports repo.

[^1]: _Technically we can, but it takes enormous effort (entire team needs to
    drop everything to spend hours working on this) and is error prone_
[^2]: _There are some tools *built* with Go to do this, but none of them have a
    stable library interface, and/or are not-production ready for other
    reasons._