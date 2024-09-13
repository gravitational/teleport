# Installation script test environment

`assets/install-scripts/install-teleport-tests` contains the test environment
for the one-line Teleport installation script available at
`https://goteleport.com/static/install.sh`.

## How the test suite works

The test suite uses `docker compose` to launch containers based on a number of
Linux distributions. Each container mounts the installation script and runs it,
then runs a command to make assertions against the results of the installation
script. 

If an assertion fails, the container running the script prints a log that
includes the string `INSTALL_SCRIPT_TEST_FAILURE`. After running containers, the
test suite looks up instances of the failure log and, if it finds any, exits
with an error code.

## Run the test suite

```bash
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

1. Add a service definition to `docker-compose.yml`.
1. Add the following bind mounts to the service definition:

   ```yaml
       volumes:
         - type: bind
           source: ../install.sh
           target: /install.sh
         - type: bind
           source: ./run-test.sh
           target: /run-test.sh
   ```

1. Edit the `command` field of the service definition to include the following:

   ```yaml
          bash /install.sh 15.0.0;
          bash /run-test.sh oss'
   ```

   Edit the parameters of the `install.sh` and `run-tests.sh` scripts as
   appropriate. The edition parameter of the two must match (the default edition
   parameter for `install.sh` is `oss`).

1. To add an assertion to the test suite, edit `run-test.sh`. Each assertion
   must print a log that includes the string `INSTALL_SCRIPT_TEST_FAILURE` if it
   fails.
