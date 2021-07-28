## Preflight Checklist

This checklist is to be run prior to cutting the release branch.

- [ ] Bump Golang vendor dependencies
- [ ] Review forked dependencies for upstream security patches
- [ ] Bump Web UI vendor dependencies
- [ ] Make a new docs/VERSION folder
- [ ] Update VERSION in Makefile to next dev tag
- [ ] Update TELEPORT_VERSION in assets/aws/Makefile
- [ ] Update mentions of the version in examples/ and README.md
- [ ] Search code for DELETE IN and REMOVE IN comments and clean up if appropriate
- [ ] Update docs/faq.mdx "Which version of Teleport is supported?" section with release date and support info
