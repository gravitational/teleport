---
authors: Zac Bergquist (zac.bergquist@goteleport.com)
state: draft
---

# RFD 170 - Active Directory Troubleshooting Tool

## Required Approvals

* Engineering: (@ibeckermayer || @probakowski) && @programmerq
* Product: @klizhentas

## What

Teleport Desktop Access relies heavily on Windows PKI in Active Directory Environments.
This RFD proposes a new troubleshooting tool that aims to identify configuration errors
and suggest next steps.

## Why

Active Directory misconfigurations generate a tremendous amount of support load for us.
Troubleshooting these misconfigurations is both difficult and time consuming, and often
requires us to schedule multiple troubleshooting calls with customers where we verify
each step of our getting started and troubleshooting guides one by one.

Additionally, Teleport logs often lack helpful troubleshooting information, as Windows
does not expose most error scenarios to the RDP client and instead writes to the local
Windows event log. This gives the impression that Teleport is fragile and hard to work
with.

A tool that performs common troubleshooting steps and writes results in a consistent
format should decrease support load, help customers to self-serve their way to success,
and restore confidence in Teleport by demonstrating that connection issues are caused
by Windows misconfigurations.

## Details

There are a variety of checks that should be performed in order to verify
whether the Active Directory environment is properly configured for Teleport.
Some of them should run from the system running the Teleport
`windows_desktop_service` to verify connectivity, and others should be run in
the Windows environment.

The checks can be divided into several categories:

- Basic networking and connectivity tests
- Windows configuration checks
- Checks for tools with known conflicts

Teleport configuration:
- w_d_s can make a secure LDAP connection (trusts and verifies the server cert)
- LDAP server trusts Teleport's client certificate (try a read and a write)
- Check whether w_d_s has reported any hosts (either via discovery or static config)
- Generate a cert with `tctl auth sign` and use `certutil` to verify it
- Check whether w_d_s has network connectivity to desktop RDP port

Windows configuration:
- Cert imported to NTAuth
- Cert exists in LDAP
- Cert exists in machine's root store
- Check registry key for user mapping
- Check KDC certs (with appropriate EKU for smart card logon)

Conflicting software tools:
- CrowdStrike (DC)
- SilverFort
- ActivID Client (target host) (check registry for smart card driver)

### Security

TBD

### Privacy

The tool will not send data anywhere. Instead, it will output the results of a series
of checks to the console, providing users an opportunity to review prior to deciding
to share with Teleport Support.

### UX

Probably drive it via `tctl` on the Windows desktop service? This allows us to
get information about the local service, invoke RPCs, and perform connection tests.

This would be a consistent experience for Cloud and Self-hosted customers, as Teleport
Cloud never runs the Windows Desktop Service.

### Proto Specification

Include any `.proto` changes or additions that are necessary for your design.

### Backward Compatibility

`tctl` will do as much client-side work as it can (network connection tests,
etc) and will print a warning advising users to upgrade the
windows_desktop_service if server-side troubleshooting RPCs are not available.

### Audit Events

Include any new events that are required to audit the behavior
introduced in your design doc and the criteria required to emit them.

### Observability

Describe how you will know the feature is working correctly and with acceptable
performance. Consider whether you should add new Prometheus metrics, distributed
tracing, or emit log messages with a particular format to detect errors.

### Product Usage

Describe how we can determine whether the feature is being adopted. Consider new
telemetry or usage events.

### Test Plan

Include any changes or additions that will need to be made to
the [Test Plan](../.github/ISSUE_TEMPLATE/testplan.md) to appropriately
test the changes in your design doc and prevent any regressions from
happening in the future.
