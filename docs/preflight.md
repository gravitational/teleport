## Preflight Checklist

This checklist is to be ran prior to cutting the release branch.

- [ ] Bump GoLang Vendor Dependencies
- [ ] Review forked Dependencies for upstream security patches
- [ ] Bump Web UI Vendor Dependencies
- [ ] Make a new docs/$VERSION folder
- [ ] Update VERSION in Makefile
- [ ] Update TELEPORT_VERSION in assets/marketplace/Makefile
- [ ] Update mentions of the version in examples/ and README.md
- [ ] Search code for DELETE IN and REMOVE IN comments and clean up if appropriate