# difftest

This tool finds tests which were changed since a previous git revision.

## Usage

To see changes:

```
difftest diff --path . --branch master
```

To get `go test` flags to run only newly appeared tests:

```
difftest test --path . --branch master --exclude-updates
```

## How it works

1. Calls `git diff $(git merge-base --fork-point <branch>)`
2. Filters all added/modified files ending with `_test.go`.
3. Builds a map of `Test*` methods in every file.
4. Calculates `SHA1` hash of each method body.
5. Does the same for the counterpart files at the fork point.
6. Compares the results. If a method was not there previously, it's marked as new. If a method's contents was changed, it is marked as changed.

## `testify/suite` support

1. Tool detects suite start signatures like the following: 

    ```
        func TestSingleSuite(t *testing.T) { suite.Run(t, &SingleSuite{}) }
    ```

    where `suite` references to `testify/suite` package, and `t` references `testing` package.

2. All methods related to a single suite must be in the same package (directory).
3. All suite parts must redside in a files matching `*_test.go` pattern.
3. If a test method has a receiver, which was not detected as a `testify/suite` previously, such method got skipped.