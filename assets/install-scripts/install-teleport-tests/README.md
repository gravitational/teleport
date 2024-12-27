# Installation script test environment

`assets/install-scripts/install-teleport-tests` contains the test environment
for the one-line Teleport installation script available at
`https://goteleport.com/static/install.sh`.

## How the test suite works

The test suite uses `docker compose` to launch containers based on a number of
Linux distributions. Each container mounts the installation script and runs it,
then runs a command to make assertions against the results of the installation
script. The test suite uses the value of the `TELEPORT_VERSION` environment
variable to determine the version of Teleport to install.

If an assertion fails, the container running the script prints a log that
includes the string `INSTALL_SCRIPT_TEST_FAILURE`. After running containers, the
test suite looks up instances of the failure log and, if it finds any, exits
with an error code.

## Run the test suite

```bash
# Assign TELEPORT_VERSION to your version number
$ export TELEPORT_VERSION=10.0.0 
$ cd assets/install-scripts/install-teleport-tests
$ bash run-all-tests.sh
```

## Run a single test case

Run the `docker compose` service that corresponds to the test case you want to
run:

```bash
$ cd assets/install-scripts/install-teleport-tests
$ docker compose up <TEST_CASE>
```

Consult `docker-compose.yml` for the available test cases.

## Add a test

Add a test by defining a service similar to the following, adjusting the values
to fit your test case:

```yaml
  test-ubuntu-jammy-cloud:
    image: ubuntu:22.04
    environment:
      - TELEPORT_VERSION
    volumes:
      - type: bind
        source: ../install.sh
        target: /install.sh
      - type: bind
        source: ./run-test.sh
        target: /run-test.sh
    # Need to install curl on the ubuntu container
    command: |
       bash -c 'apt-get update;
       apt-get install -y curl;
       bash /install.sh ${TELEPORT_VERSION} cloud;
       bash /run-test.sh cloud'
```

Edit the parameters of the `install.sh` and `run-tests.sh` scripts as
appropriate. The edition parameter of the two must match (the default edition
parameter for `install.sh` is `oss`).

To add an assertion to the test suite, edit `run-test.sh`. Each assertion must
print a log that includes the string `INSTALL_SCRIPT_TEST_FAILURE` if it fails.
