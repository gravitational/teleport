## Differences between deb & RPM scripts

During an upgrade of a deb package from one version to another, the following steps happen:

1. The new version is unpacked, with the new files overwriting the old files.
2. The after-remove of the old version is called.
3. The old files are removed.
4. The after-install of the new version is called.

See:

- [Debian Policy Manual: Upgrading a package](https://www.debian.org/doc/debian-policy/ap-flowcharts.html#id5).
- [Debian Policy Manual: Details of unpack phase of installation or
  upgrade](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html#details-of-unpack-phase-of-installation-or-upgrade)

However, when you upgrade an RPM package, the scriptlets are called in a reverse order:

1. The new version is unpacked, with the new files overwriting the old files.
2. The after-install of the new version is called.
3. The old files are removed.
4. The after-remove of the old version is called.

See [Fedora Docs: Scriptlets - Ordering](https://docs.fedoraproject.org/en-US/packaging-guidelines/Scriptlets/#ordering).

This has direct consequences when attempting to use the same scripts for both targets.
