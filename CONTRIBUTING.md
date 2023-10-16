# Contributing A Patch

If you're interesting in working on an existing issue, such as one of the
[`good-starter-issue`](https://github.com/gravitational/teleport/issues?q=is%3Aopen+is%3Aissue+label%3Agood-starter-issue),
ones, simply respond to the issue and express interest in working on it. This
helps other people know that the issue is active, and hopefully prevents
duplicated efforts.

If you want to work on a new idea of relatively small scope:

1. Submit an issue or comment on an existing issue describing the proposed
   change and the implementation.
2. The repo owners will respond to your issue promptly.
3. If your proposed change is accepted, fork the repository.
4. Write your code, test your changes and _communicate_ with us as you're moving forward.
5. Submit a pull request.
6. Teleport maintainers will review your code, provide feedback, and ultimately create a
   buddy PR incorporating your changes.

## Adding dependencies

If your patch depends on new packages, the dependencies must:

- be approved by core Teleport contributors ahead of time
- use an approved license
- Go dependencies must be using [Go modules](https://blog.golang.org/using-go-modules)

## Helm

Instructions for contributing changes to the Teleport Helm chart are available
[here](./examples/chart/CONTRIBUTING.md).

# Contributing to Docs

See our public resources for docs contributors:
https://goteleport.com/docs/contributing/documentation/
