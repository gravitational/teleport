clockwork
=========

[![Build Status](https://travis-ci.org/jonboulle/clockwork.png?branch=master)](https://travis-ci.org/jonboulle/clockwork)
[![godoc](https://godoc.org/github.com/jonboulle/clockwork?status.svg)](http://godoc.org/github.com/jonboulle/clockwork)

a simple fake clock for golang

# Usage

Replace uses of the `time` package with the `clockwork.Clock` interface instead.

For example, instead of using `time.Sleep` directly:

```go
func myFunc() {
	time.Sleep(3 * time.Second)
	doSomething()
}
```

inject a clock and use its `Sleep` method instead:

```go
func myFunc(clock clockwork.Clock) {
	clock.Sleep(3 * time.Second)
	doSomething()
}
```

Now you can easily test `myFunc` with a `FakeClock`:

```go
func TestMyFunc(t *testing.T) {
	c := clockwork.NewFakeClock()

	// Start our sleepy function
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		myFunc(c)
		wg.Done()
	}()

	// Ensure we wait until myFunc is sleeping
	c.BlockUntil(1)

	assertState()

	// Advance the FakeClock forward in time
	c.Advance(3 * time.Second)

	// Wait until the function completes
	wg.Wait()

	assertState()
}
```

and in production builds, simply inject the real clock instead:

```go
myFunc(clockwork.NewRealClock())
```

See [example_test.go](example_test.go) for a full example.

# Credits

clockwork is inspired by @wickman's [threaded fake clock](https://gist.github.com/wickman/3840816), and the [Golang playground](https://blog.golang.org/playground#TOC_3.1.)
