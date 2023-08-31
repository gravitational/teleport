This is a dummy Go module for the purposes of generating test data for
render-test. Run:

    go test -cover -json ./...

Modify some of the test files to make them fail or to skip tests to
vary the data. The output is stored in files in the `../testdata/`
directory.
