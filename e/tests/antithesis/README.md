# Teleport Antithesis Tests

This directory contains the Docker images, SUT definitions, workload code, and
launch helper used to run Teleport under Antithesis.

## Structure

- `docker/`: Common container builds shared by SUTs.
- `sut/`: System Under Test definitions. Each SUT owns its compose file, config,
  params and image list.
- `workloads/`: Antithesis workload commands used by workload images.

## Workloads

Antithesis discovers commands by path and filename prefix inside a test
template. Follow the Antithesis docs when adding or renaming commands:

- [Test commands](https://antithesis.com/docs/test_templates/test_composer_reference)
- [Go lifecycle package](https://antithesis.com/docs/generated/sdk/golang/lifecycle)
- [Go assert package](https://antithesis.com/docs/generated/sdk/golang/assert)
- [Go random package](https://antithesis.com/docs/generated/sdk/golang/random)

Note that each workload container can contain multiple test templates as explained by the [docs](https://antithesis.com/docs/test_templates/first_test/#using-multiple-test-templates):

```
/opt/antithesis/test/v1/
├── basic/                            <- simple producer/consumer tests
│   ├── first_create_topic.py
│   ├── parallel_driver_produce.py
│   └── parallel_driver_consume.py
├── transactional/                    <- transaction-focused tests
│   ├── first_setup_tx.py
│   ├── parallel_driver_tx_write.py
│   └── finally_tx_verify.py
└── stress/                           <- high-throughput stress test
    └── singleton_driver_load.py
```

Any containers may contain these binaries, and the test template name is shared among all of them for the
purpose of the run. Antithesis will then select one of the test templates per execution history. This allows multiple
testing strategies within a single Antithesis run.

## Environment

Create `.env` from `.env.example`
before building or triggering runs. Set the Antithesis tenant, launcher,
registry, and builder values for your environment. For local-only runs,
`REGISTRY` can be empty.

## Local iteration

From the `e/tests/antithesis` path, build and load the example images into Docker, write
the k3s image archives, then write the SUT env file used by Docker Compose:

```shell
make local build-k3s-preload write-env
```

The preload target writes `sut/core/k3s/preload/nginx.tar` so k3s can import the
nginx image without pulling from a registry at runtime.

Start the stack from this SUT directory:

```shell
cd sut/core
docker compose up
```

In another terminal, run the workload command:

```shell
cd sut/core
docker compose exec workload /opt/antithesis/test/v1/crud/parallel_driver_crud_fuzzer
```

## Trigger Antithesis

Build and push the images, then trigger the core run:

```shell
make build-all trigger SUT=core
```
This target runs the `trigger.sh` script, note that by default credentials in `~/.netrc` are used.

## Contributing

To add a testsuite to Antithesis either:

1. Create a SUT, you can copy the `core` as a starting point. Or:
2. Modify an existing SUT to add a new test template.

Generally unless the test requires a different deployment strategy, configuriation or backend it is instead
best to extend an existing SUT.

### Adding a new SUT

1. The SUT is defined as a `docker-compose.yaml` file. This is packaged into a config container by placing all files required into a docker container. Ensure all required configs are correctly placed.
2. Tweak `params.json` to update the description, duration and any settings for fault injection. Ensure `antithesis.source` matches the SUT name, this seperates property history between runs of different SUTs within Antithesis to keep track of resolutions and history in the GUI.
3. Update `images.sh` to contain the list of any containers that are required by the SUT. Update the name of the `CONFIG_IMAGE` used.

### Extending workloads

All workload commands are built within the same container as instrumented Teleport itself since the definitions are in-tree.

1. Add any new commands to `workloads/cmd`.
2. Extend the build `e/tests/antithesis/docker/teleport/Dockerfile` to build the required binary.
3. Use the shared workload runtime stage to place the required binaries into the workload image for a given SUT.
