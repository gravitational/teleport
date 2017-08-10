### Contributing A Patch

If you're working on an existing issue, such as one of the `help-wanted` ones
above, simply respond to the issue and express interest in working on it.  This
helps other people know that the issue is active, and hopefully prevents
duplicated efforts.

If you want to work on a new idea of relatively small scope:

1. Submit an issue describing the proposed change and the implementation.
2. The repo owners will respond to your issue promptly.
3. If your proposed change is accepted, fork the repository.
4. Write your code, test your changes and _communicate_ with us as you're moving forward.
4. Submit a pull request.

### Adding dependencies

If your patch depends on new packages, the dependencies must:

- be licensed via Apache2 license
- be approved by core Teleport contributors ahead of time
- a dependency package must be vendored via [`godep`](https://github.com/tools/godep)
