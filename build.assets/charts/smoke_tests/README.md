# Docker Image Smoke Tests

This directory contains some smoke tests for our Docker images. Somke tests are 
quick go-or-no-go tests to check and see if the images we're shipping meet some 
basic functionality standard.

At the time of writing, that statndard is pretty low:
 1. does `teleport` start at all, and
 2. does the `PAM` soubsystem come up when loaded by `teleport`. 

How do we raise the bar? More and better tests!

## Writing a smoke test

1. Create a directory under this `smoketest` root.
2. Add an executable bash script in this directory. This script must
    1. be named `test.sh`
    2. be executable (i.e. has `+x` permissions),
    3. take a two arguments: 
       1. the name of the docker image to test, and
       2. the platform to test it on
    4. return `0` on success, nonzero on error


