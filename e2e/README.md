## Teleport E2E Tests

This directory contains end-to-end tests for Teleport. These tests are
designed to be run against a live cluster. They are written in TS and use
[Playwright](https://playwright.dev/) to interact with the browser.
Docker compose is used to spin up a cluster for testing and to run the tests.

### Running the tests
```bash
# Make all removes the existing docker volumes to ensure a clean state
# and rebuild the containers
make all 
```
or

```bash
# Only run tests
make test
```

### MacOS building notes

Before running the tests on MacOS in Docker, you need to build Linux compatible binaries.
Binaries are build using our Docker images inside `build.assets` directory. You can also
build them manually using `make build-binaries` command.

### Running tests for development

Docker compose setup is designed to run tests in CI and create the same environment
locally, so debugging potential issues is easier. E2E tests make changes to the cluster,
so the order of the tests is important. To run tests for development, you can use
`yarn test` command to run the test against the existing cluster.
`yarn codegen` starts the Playwright codegen tool that allows to record the test
and generate the code for it. This improves the development speed as most code can be generated.

### Common issues

`Cannot run macOS (Mach-O) executable in Docker: Exec format error`  

This error means that you are trying to run MacOS binary on Linux. You need to build
Linux compatible binaries to run them in Docker. You can rebuild them using `make build-binaries`
or just remove existing binaries in `../build` as they will be rebuilt automatically.
