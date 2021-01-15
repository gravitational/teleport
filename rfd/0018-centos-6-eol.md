---
authors: Gus Luxton (gus@goteleport.com)
state: draft
---

# RFD 18 - EOL for Teleport CentOS 6 binaries

## What

The Teleport build pipeline currently builds special CentOS 6 binaries (which are linked
against glibc 2.12) for customers who have not yet upgraded from CentOS 6.

We will no longer produce CentOS 6 binaries for the next major version of Teleport (6.0).

## Why

CentOS 6 was originally released in July 2011. Full updates for all its packages stopped
in May 2017, and [CentOS 6 has been officially declared EOL](https://wiki.centos.org/FAQ/General#What_is_the_support_.27.27end_of_life.27.27_for_each_CentOS_release.3F)
as of November 2020.

The [CentOS 6 official package mirrors have been taken down](http://mirror.centos.org/centos-6/6/readme)
and the message being shared here by the maintainers could not be clearer - `"The whole CentOS 6 is *dead* and *shouldn't* be used anywhere at *all*"`

Using EOL distros is unsafe and even dangerous for many reasons. CVEs and security bugs are regularly found
and never fixed, leaving servers insecure and easily pwned by trivial exploits.

Continuing to support CentOS 6 is undesirable:
- Teleport is fundamentally positioned as a security product - its objective is to provide secure access
  to infrastructure. Continuing to build binaries for an end-of-life distribution sends a bad message;
  that Teleport is willing to support the use of end-of-life distributions which do not receive any
  security updates.
- Having separate special-cased builds increases the size and complexity of Teleport's build
  matrix, often requiring workarounds to support features which newer distributions provide out of the box.
- Some Teleport features (for example: enhanced session recording) do not work on CentOS 6 because
  the kernels used are too old. Inconsistencies in the Teleport experience across distributions are
  bad for product messaging and potential customers.
- Maintaining our own copy of the CentOS 6 package repos for any length of time just to create builds
  will add cost, complexity and more potential for failures.
- Every time we release a new major version of Teleport with CentOS 6 binaries available, we are forced us to
  keep outdated infrastructure around for a minimum of one year from the date when that version is released.

Removing CentOS 6 builds will:
  - Make testing of future Teleport releases easier and more consistent by decreasing the number of different
    permutations that must be tested
  - Allow us to test and link all built binaries against the same version of glibc, increasing confidence
    in compatibility
  - Reduce the complexity of our download page, allowing people to find the correct binaries more easily
  - Make providing Teleport support easier for the customer success team
  - Make writing documentation and examples easier, due to increased commonality between environments
    (for example: CentOS 6 does not use systemd, whereas CentOS 7+, Amazon Linux, Fedora, SuSE and other
    RHEL variants do)
  - Incentivize users to use a maintained software stack without known unpatched vulnerabilities

## Details

Teleport 6.0+
- Teleport 6.0+ will be released without CentOS 6 binaries available.
- We will notify all customers ASAP (via Zendesk ticket) that CentOS 6 binaries will no longer
  be available for Teleport 6.0+, to give them as much notice of this change as possible.
  
Teleport 5.x
- We will continue to build CentOS 6 binaries with security fixes for all released Teleport 5.x versions.
- We will continue to support Teleport 5.x on CentOS 6 [until November 2021
  as per our FAQ](https://goteleport.com/teleport/docs/faq/).
