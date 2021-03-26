---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: implemented
---

# RFD 1 - Testing guidelines

## What

Guidelines for writing Go tests for teleport.

## Why

At the time of writing this, teleport tests use
[gocheck](https://labix.org/gocheck). This framework has several downsides:
- all tests are serial - slow execution in packages with many small tests
- easy to mess up setup (forgetting to call `check.T`) or run tests multiple
  times (calling `check.T` multiple times)
- limited deep-diffing of values
- largely unmaintained

At the same time, there are some features we'd like to retain:
- organization of tests into testsuites
- shared setup/teardown code between tests
- asserts for concise validation

Go's builtin `testing` package covers most of our needs, and gets new features
with every new Go release. We just need to complement it with:
- asserts
- flexible value diffing (got vs want)

## Details

Use the `testing` package as a base and complement it with several libraries.

### Test organization

Use a hierarchy of:
- `_test.go` files
- `func Test*` functions
- [`t.Run`](https://golang.org/pkg/testing/#hdr-Subtests_and_Sub_benchmarks)
  subtests (aka table-driven tests)

Shared setup/teardown:
- [`func TestMain`](https://golang.org/pkg/testing/#hdr-Main) for package level
- a single `func Test*` with subtests for smaller test groups
- `func setupX` called from multiple `func Test*` for larger test groups

For performance testing use
[benchmarks](https://golang.org/pkg/testing/#hdr-Benchmarks).

### Parallelization

Call [`t.Parallel()`](https://golang.org/pkg/testing/#T.Parallel) from all
tests and subtests that don't need to run serially.

### Asserts

Use
[testify/require](https://pkg.go.dev/github.com/stretchr/testify/require?tab=doc)
or a plain `if` condition, whichever is easiest.

Skim through the `testify/require` docs, it has many convenient helpers.

### Diffing

For trivial comparisons, use helpers from
[testify/require](https://pkg.go.dev/github.com/stretchr/testify/require?tab=doc).

For non-trivial comparisons (such as deeply-nested structs or protobufs), use
[go-cmp](https://pkg.go.dev/github.com/google/go-cmp/cmp?tab=doc) with
[cmpopts](https://pkg.go.dev/github.com/google/go-cmp@v0.5.1/cmp/cmpopts?tab=doc).

It allows you to customize checking by ignoring types/fields, equating empty
and nil slices, approximating float comparisons. See examples below.

### Wishlist

Some things we don't have a good solution for yet:
- log output collection
  - if something is logged by a function under test and that test fails, print
    the log output
  - it's currently hard to separate logging from a function under test and
    unrelated logs by a background goroutine
- performance regression tracking
  - `go test` will report runtimes of individual tests/subtests
  - but we don't have tooling to track those over time
  - note: [benchmarks](https://golang.org/pkg/testing/#hdr-Benchmarks) are an
    alternative, but must be written manually

## Migration

Re-writing all of teleport tests is a large task. To make it feasible,
migration will happen organically over the course of ~1 year after these
guidelines are approved.

The approach is:
1. no unit tests are rewritten upfront
1. any new test should follow these guidelines
1. updates to any existing test (as part of PRs) should be bundled with a
  migration of that test

There are a few exceptions to #1:
- `/integration` tests
- `/lib/services/...` tests

These are massive test suites that are ripe for refactoring. Rewriting them
will help us flesh out any unforeseen issues with the new testing guidelines.

If after 1 year we're left with un-migrated tests, we will discuss whether it's
worth to invest into dedicated rewrite.

### Examples

Subtests

```go
func TestParseInt(t *testing.T) {
	tests := []struct {
		desc      string
		in        string
		want      int
		assertErr require.ErrorAssertionFunc
	}{
		{desc: "positive", in: "123", want: 123, assertErr: require.NoError},
		{desc: "negative", in: "-123", want: -123, assertErr: require.NoError},
		{desc: "non-numeric", in: "abc", assertErr: require.Error},
		{desc: "empty", in: "", assertErr: require.Error},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := parseInt(tt.in)
			tt.assertErr(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}
```

`go-cmp` ignoring fields

```go
type Foo struct {
	A int
	B Bar
}

type Bar struct {
	C    string
	Time time.Time
}

func TestParseInt(t *testing.T) {
	x := Foo{A: 1, B: Bar{C: "one", Time: time.Now()}}
	y := Foo{A: 1, B: Bar{C: "one", Time: time.Now().Add(time.Minute)}}

	require.Empty(t, cmp.Diff(x, y, cmpopts.IgnoreFields(Bar{}, "Time")))
}
```

`TestMain` shared setup

```go
func TestMain(m *testing.M) {
	// Setup
	//
	// Note: TestMain can only use ioutil.TempDir for temporary directories.
	// For actual tests, use t.TempDir().
	tmpDir, err := ioutil.TempDir("", "teleport")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Run tests
	result := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)

	// Done
	os.Exit(result)
}
```

Shared test setup/teardown

```go
func expensiveTestSetup(t *testing.T) *Foo {
	tmp := t.TempDir()
	
	f, err := newFoo(tmp)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	
	return f
}

func TestFoo1(t *testing.T) {
	f := expensiveTestSetup(t)

	// Test something with f.
}

func TestFoo2(t *testing.T) {
	f := expensiveTestSetup(t)

	// Test something else with f.
}
```
