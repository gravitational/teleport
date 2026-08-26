# Docker Image Smoke Tests

This directory contains some smoke tests for our Docker images. Smoke tests are 
quick go-or-no-go tests to check and see if the images we're shipping meet some 
basic functionality standard.

At the time of writing, that statndard is pretty low:
 1. does `teleport` start at all, and
 2. does the `PAM` soubsystem come up when loaded by `teleport`. 

How do we raise the bar? More and better tests!

## Running smoke tests

```bash
$ ./run $PLATFORM $IMAGE_UNDER_TEST $TELEPORT_RELEASE
```
Where
 * `$PLATFORM` is the platform to test, in a format acceptable to `docker run`
 * `$IMAGE_UNDER_TEST` is the URL of the image to test
 * `$TELEPORT_RELEASE` is the release of teleport included in the
   `$IMAGE_UNDER_TEST` image. Must be either `oss` or `enterprise`.

## Writing a smoke test

1. Create a directory under this `smoketest` root.
2. Add an executable bash script in this directory. This script must
    1. be named `test.sh`
    2. be executable (i.e. has `+x` permissions),
    3. take three arguments: 
       1. the name of the docker image to test, and
       2. the platform to test it on
       3. The release of Teleport contained in the image, either `oss` or `enterprise`.
    4. return `0` on success, nonzero on error


