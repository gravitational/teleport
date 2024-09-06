---
authors: Tim Ross (tim.ross@goteleport.com)
state: draft
---

# RFD 153 - Logging Guidelines

## Required approvers

- Engineering: `@zmb3`

## What

Guidelines for logging within Teleport components.

## Why

At the time of writing this, Teleport uses [logrus](https://github.com/sirupsen/logrus) for structured logging. This
library is in maintenance mode, and while not completely dead, updates are limited to security vulnerabilities,
(backwards compatible) bug fixes, and performance (where we are limited by the interface). As of Go 1.21, the standard
library now includes its own structured logger: [log/slog](https://pkg.go.dev/log/slog). While there are other logging
libraries that have tried to replace `logrus` none of them have ever gained consensus and widespread adoption within the
community. `log/slog` may not be perfect, but the fact that it's included in the standard library and was designed to be
far more flexible than the `log` package, which led to libraries like `logrus` in the first place, makes it a preferred
choice.

All logging performed in the `api` module is also done via `logurs`. Not only does the dependency inflate the size of
the `api` module, it also forces any consumers of the package to instantiate a default `logurs` logger to see any
output. This forces consumers to also import `logrus` even though they may prefer to use a different logging library. By
using`log/slog` instead we can allow consumers of the `api` module to use whatever `slog.Handler` they would like to
output any logs from `api`.

## Details

Converting logging libraries is not going to be a quick and easy process, but it also does not have to be done in a
single change. Both `logrus` and `log/slog` can coexist within Teleport until all uses of `logrus` can be migrated. The
most important thing is for consumers of the logs not to be able to tell which library produced the output. This means
that we cannot use the builtin `slog.TextHandler` and `slog.JSONHandler` and instead must write custom handlers that
match the output from our
[`logrus.TextFormatter`](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/utils/formatter.go)
implementations.

Once we can produce output from `log/slog` that matches `logrus` we can begin the following migration process.
Initialize a slog and logurs logger in both the
[global logging initialization](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/utils/cli.go#L50-L70)
and the
[`servicecfg` logging initialization](https://github.com/gravitational/teleport/blob/004d0db0c1f6e9b312d0b0e1330b6e5bf1ffef6e/lib/config/configuration.go#L652-L725).
Since both loggers would be writing to the same output (`os.Stdout`, `os.Stderr`) we need to ensure that messages don't
clobber each other. To prevent this a simple `io.Writer` wrapper that guards calls to write with a mutex and then writes
to the configured output can be used.

Once a `slog.Logger` may be retrieved either globally via `slog.Default`/`slog.With` or by dependency injection, the
process of converting individual loggers may begin. Since `TeleportProcess` is the entry point and object responsible
for creating other components it should be the first component to be converted to have and use a `slog.Logger`. Once it
has been converted, objects it creates can be migrated independently until they've all been converted. The `api`module
consumes the global `logrus` logger, so it may be converted independently after the global `slog.Logger` is configured
in the first step. However, the minimum supported go version of the `api` module will need to be bumped to >= go 1.21
before package `log/slog` is available to be consumed there. When the last consumers of the `logrus` library have been
removed from the `api` module `go mod tidy` to get rid of the `logrus` dependency (or at least make it an indirect
dependency). Once `slog` is the only library writing output from within `teleport` the log initialization logic can be
cleaned up to only create a `slog.Logger` and remove the `io.Writer` wrapper from the first step. At this
point `go mod tidy` should be run to clean up the dependency. A depguard rule should also be put in place to prevent any
accidental uses of `logrus` after the conversion.

### API Differences

There are a few subtle API differences between the two logging libraries which prevent migrating to be a find and
replace operation. The following sections demonstrate these differences and offer various recommendations to overcome
them when migrating to `log/slog`.

#### Formatted messages

`logrus` provides an API which allows for `fmt.Sprintf` style formatting (Tracef,Debugf,Infof, etc.) of log messages
which are relied on heavily throughout the codebase.

> ```go
> logrus.Infof("Connected to cluster %v at %v.", clusterName, conn.RemoteAddr())
>```
>```bash
>2023-11-02T15:18:40-04:00 [PROXY]    INFO Connected to cluster foo at 192.168.1.243. pid:14968.1  regular/proxy.go:250
>```

The `log/slog` API differs in that all logging functions provided take a message and a series of key value pairs,
referred to as attributes, instead of a message to format and associated arguments. Any attributes provided will be
included as fields alongside the message. While a literal translation of the above call to `Infof` is possible in slog
by using `fmt.Sprintf` directly it is not recommended. Instead, when converting to `log/slog` structured logging should
be embraced. The message from above can be translated to carry a bit more context, and the additional fields in the
message can be moved to the attributes.

> ```go
> slog.InfoContext(ctx, "Proxied connection to cluster established", "cluster_name", clusterName, "remote_address", conn.RemoteAddr())
>```
>```bash
>2023-11-02T15:18:40-04:00 [PROXY]    INFO Proxied connection to cluster established pid:14968.1 cluster_name:foo remote_address:192.168.1.243  regular/proxy.go:250
>```

#### WithField(s)

To add structured fields with `logrus` either the `WithField` or `WithFields` methods were used, the only difference
being the latter took a map of fields instead of a single field.

```go
logrus.WithField("user", user).Warn("Failed to emit account recovery code used failed event")
```

`log/slog` provides similar functionality via the `With` method, though the API differs slightly. Just like the logging
API, `With` takes a series of key value pairs instead of a struct like `logrus`. The above example can naively be
converted to the following with `log/slog`.

```go
slog.With("user", user).WarnContext(ctx, "Failed to emit account recovery code used failed event")
```

However, instead of creating a clone of the logger with the `user` field added, the desired attributes should be
passed directly to the `Warn` method instead.

```go
slog.WarnContext(ctx, "Failed to emit account recovery code used failed event", "user", user)
```

The main benefit to using `slog.With` is that any attributes provided to it are formatted once instead of each time a
message is logged. `slog.With` should only be used when enriching a logger with additional information that is used to
log several messages, be that in a function or a logger that is a struct member.

> `logrus`
>```go
>logrusLogger := logrus.WithFields(logrus.Fields{
>"device_id": ref.DeviceID,
>"os_type":   ref.OSType,
>"asset_tag": assetTag,
>})
>...
>logrusLogger.Debug("some message")
>...
>logrusLogger.Warn("some other message")
>```
>`log/slog`
>```go
>slogLogger := slog.With("device_id", ref.DeviceID, "os_type", ref.OSType, "asset_tag", assetTag)
>...
>slogLogger.DebugContext(ctx, "some message")
>...
>slogLogger.WarnContext(ctx, "some other message")
>```

#### WithError

Unlike `logrus`, `log/slog` does not provide any special APIs for dealing with errors. When converting to `log/slog` any
usages of `WithError` should follow the same advice from the `WithField(s)` section. If the error is only included in a
single log message, then it should be added as an attribute to the log function. If the error is included in multiple
messages then a logger should be created via `With("error", err)`.

> `logrus`
>```go
>logrus.WithError(err).Error("Error parsing response.")
>```
>`log/slog`
>```go
>slog.ErrorContext(ctx, "Error parsing response", "error", err)
>```

### Best Practices

If many log lines share a common attribute, use a `slog.With` to construct a `slog.Logger` with that attribute. This
allows it to only be formatted once instead of every time a log is emitted.

If an attribute is only emitted in a single log entry, prefer passing it as an attribute instead of
calling `slog.With`.

Arguments to a logging function are always evaluated, even if the log event is discarded due to configured verbosity.
If the object implements `fmt.Stringer` you can pass it by pointer and the `String` method will only be called when
the log is being written. Another option is to implement
the [`slog.LogValuer`](https://pkg.go.dev/log/slog@master#LogValuer) interface.

For high volume log messages prefer to use `slog.Logger.LogAttrs`. While the API is more verbose, it avoids any
allocations, making it the most efficient way to produce output. The example below illustrates how to convert a
message using `InfoContext` to `LogAttrs`.

```go
slog.InfoContext(ctx, "Speed threshold reached", "flux_capacitors_charged", true, "operator", "doc brown")
slog.LogAttrs(ctx, slog.LevelInfo, "Speed threshold reached", slog.Bool("flux_capacitors_charged", true), slog.String("operator", "doc brown"))
```


Package global loggers should not use `slog.With` to create the logger. Doing so results in incorrect formatting
because the default slog handler is used instead of the slog handler set at runtime (see 
https://github.com/gravitational/teleport/issues/40629 for more details). Instead a global logger should be
created with `logutils.NewPackageLogger` which allows formatting of attributes to be deferred until runtime
so that the correct formatter is used.

```go
var logger = logutils.NewPackageLogger(teleport.ComponentKey, "llama")
```


### Logging Standards

There currently exists no logging standards within Teleport, which has resulted in various different patterns depending
on which file you open. If we are going to take the time to adjust all existing log messages while converting
to `log/slog`, we should do our best to start following a few simple rules.

1) Embrace structured logging. Prefer using static messages and attributes instead of relying on `fmt.Sprintf` style
   formatting. Enrich log messages with additional attributes to provide additional context.
1) All keys must be snake case to ensure all output from Teleport is consistent.
1) Use the Context variants of the `slog.Logger` API. Providing the `context.Context` to all log messages allows them
   to be enriched with additional attributes. For example, this allows log messages to automatically have a request or
   trace id added, which makes correlating messages that were emitted as a result of one particular action or event.
   If no context is readily available for the logger to consume, prefer to use a `context.TODO` if it is possible to get
   a context to the call site in the future but refactoring is to do be done in a separate unit of work. If there is
   likely to never be a context provided to the function producing the log output, then prefer to use
   a `context.Background` to indicate as such.
1) The message should be a fragment, and not a full sentence. Terminating the message with punctuation should be
   avoided.

To achieve these, we can add enable the [`sloglint`](https://github.com/go-simpler/sloglint) linter in our golangci-lint
configuration.

```yaml
linters:
  enable:
    - sloglint

linters-settings:
  sloglint:
    static-msg: true
    key-naming-case: snake
    context-only: true
```

### Security

Any CVEs identified with `log/slog` will be addressed by the Go team in a timely manner. The same cannot be said
for `logrus`, at the time of this writing there has been an issue open for more than a week reporting that it is
vulnerable to [CVE-2022-28948](https://github.com/sirupsen/logrus/issues/1406) without so much as a reply.

### Backward Compatibility

It is not recommended to backport the changes suggested in this RFD. Only `branch/v14` and newer are built with Go 1.21,
which means it is the only branch even capable of using `log/slog`, we could use `x/exp/slog` for the other release
branches. If this causes an increased number of conflicts when backporting changes to release branches, we can
reconsider backporting the efforts required to change logging libraries.

### Alternative migration strategies

#### Alternative 1

Provide the `log/slog` API with a wrapped type that used `logrus` under the hood.

```go
type slogrus struct {
wrapped logrus.FieldLogger
}

// InfoContext here matches slog's InfoContext signature
func (s *slogrus) InfoContext(ctx context.Context, msg string, args ...any) {
s.wrapped.WithFields(convertToFields(args)).Info(msg)
}
```

We would slowly migrate all `logrus` callsites to use this new wrapper, and then, upon completion, swap out the wrapper
for `log/slog`.

Positives:

- Only a single logger implementation actually writing to the output. This reduces concerns about handling concurrent
  writes, but also simplifies debugging as we will know which implementation wrote a line.

Negatives:

- The conversion wrapper will be more complex than it first seems to write. Ensuring that it remains performant and can
  handle Attributes/Values properly will be riddled with potential edge cases.
- Most potential benefits of switching to slog will not be realized until the final cut over, this may make it
  harder to achieve the buy in from engineers which will be necessary to start migrating.
- There will be a large time investment to write the wrapper and ensure it works only to throw it away after the
  migration.

#### Alternative 2

Write a `slog.Handler` that uses a `logrus.Logger` to format and produce output.

```go
type slogrus struct {
wrapped logrus.FieldLogger
}

func (s *slogrus) WithAttrs(attrs []Attr) Handler {
return s.wrapped.WithFields(convertToFields(args))
}

func (s *slogrus) Handle(_ context.Context, r Record) error {
s.wrapped.WithFields(convertRecordToFields(r)).Log(convertLevel(r.Level), r.Message)
return nil
}
```

Positives:

- Like with option 1, only a single logger implementation is writing messages to the output.

Negatives:

- There will be a large time investment to write the handler and ensure it works only to throw it away after the
  migration.
- A handler which doesn't use logrus, but still produces the same output format as logrus will still need to be written.
