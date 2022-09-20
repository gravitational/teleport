package gofakes3

import "time"

type Option func(g *GoFakeS3)

// WithTimeSource allows you to substitute the behaviour of time.Now() and
// time.Since() within GoFakeS3. This can be used to trigger time skew errors,
// or to ensure the output of the commands is deterministic.
//
// See gofakes3.FixedTimeSource(), gofakes3.LocalTimeSource(tz).
func WithTimeSource(timeSource TimeSource) Option {
	return func(g *GoFakeS3) { g.timeSource = timeSource }
}

// WithTimeSkewLimit allows you to reconfigure the allowed skew between the
// client's clock and the server's clock. The AWS client SDKs will send the
// "x-amz-date" header containing the time at the client, which is used to
// calculate the skew.
//
// See DefaultSkewLimit for the starting value, set to '0' to disable.
//
func WithTimeSkewLimit(skew time.Duration) Option {
	return func(g *GoFakeS3) { g.timeSkew = skew }
}

// WithMetadataSizeLimit allows you to reconfigure the maximum allowed metadata
// size.
//
// See DefaultMetadataSizeLimit for the starting value, set to '0' to disable.
func WithMetadataSizeLimit(size int) Option {
	return func(g *GoFakeS3) { g.metadataSizeLimit = size }
}

// WithIntegrityCheck enables or disables Content-MD5 validation when
// putting an Object.
func WithIntegrityCheck(check bool) Option {
	return func(g *GoFakeS3) { g.integrityCheck = check }
}

// WithLogger allows you to supply a logger to GoFakeS3 for debugging/tracing.
// logger may be nil.
func WithLogger(logger Logger) Option {
	return func(g *GoFakeS3) { g.log = logger }
}

// WithGlobalLog configures gofakes3 to use GlobalLog() for logging, which uses
// the standard library's log.Println() call to log messages.
func WithGlobalLog() Option {
	return WithLogger(GlobalLog())
}

// WithRequestID sets the starting ID used to generate the "x-amz-request-id"
// header.
func WithRequestID(id uint64) Option {
	return func(g *GoFakeS3) { g.requestID = id }
}

// WithHostBucket enables or disables bucket rewriting in the router.
// If active, the URL 'http://mybucket.localhost/object' will be routed
// as if the URL path was '/mybucket/object'.
//
// See https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html
// for details.
func WithHostBucket(enabled bool) Option {
	return func(g *GoFakeS3) { g.hostBucket = enabled }
}

// WithoutVersioning disables versioning on the passed backend, if it supported it.
func WithoutVersioning() Option {
	return func(g *GoFakeS3) { g.versioned = nil }
}

// WithUnimplementedPageError allows you to enable or disable the error that occurs
// if the Backend does not implement paging.
//
// By default, GoFakeS3 will simply retry a request for a page of objects
// without the page if the Backend does not implement pagination. This can
// be used to enable an error in that condition instead.
func WithUnimplementedPageError() Option {
	return func(g *GoFakeS3) { g.failOnUnimplementedPage = true }
}
