---
authors: Joel Wejdenstal (jwejdenstal@goteleport.com)
state: draft
---

# RFD 76 - Data Scalable API's

## What

Design and implement patterns for making API's that deal with unbounded data resources (e.g. session trackers, nodes, apps) scalable in terms of memory usage and with respect to gRPC maximum message sizes.

## Why

Teleport is ridden with API's, both internal over `auth.ClientI` and external over gRPC, that take the form of `func (*Service) (...params) ([]Resource)`. That is, they consist of a single function call that with some input, and return a potentially unbounded number of resources. This is problematic for a number of reasons we have encountered in Teleport as we continue to scale.

There are two major concerns with this pattern:
- The API is not designed to be scalable in terms of memory usage. This means that the larger this list grows, the more peak memory Teleport requires which quickly becomes unpredictable and can lead to OOM errors.
- gRPC enforces maximum message sizes due to it's internal protocol, because of this, Teleport may run into size errors once the list grows too large, effectively resulting in a cluster-wide DOS attack that we have to hot-patch to make the cluster operable again.

## Details

### General

The core of this proposal revolves around switching from a design paradigm based on buffer pull to one based on iterator pull. This is very similar to the [Iterator concept in Rust](https://doc.rust-lang.org/std/iter/trait.Iterator.html). The idea is that the API is designed to be scalable in terms of memory usage by returning iterator objects instead of arrays. Iterator objects are by design lazy and allow the caller to only fetch the amount of data they need and to transform the data further without dealing with large slices.

### Internal API's

Since Go is rarely used this way, I've had to prototype my own [Iterator-like library in Go](https://github.com/xacrimon/functional) that implements a subset of the [Iterator concept in Rust](https://doc.rust-lang.org/std/iter/trait.Iterator.html). Using this design, I redesigned the [GetActiveSessionTrackers](https://github.com/gravitational/teleport/blob/ca520999c1f3e929e98f37c551532446bfbfbbd7/lib/services/sessiontracker.go#L33) to transform it from a signature of `GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error)` to `GetActiveSessionTrackers(ctx context.Context) Iter[Result[types.SessionTracker]]`.

#### Interfaces and New Types

This new API signature contains theww foreign types that are new in this concept:
- `Result[T]`: A type agnostic wrapper that acts like a tagged union, contains either a value of type `T` or an error.
- `Option[T]`: A type agnostic wrapper that acts like a tagged union, contains either a value of type `T` or nothing at all. This is fundamentally different to `*T` as this type need not be allocated on the heap but can still represent nothing.
- `Iter[T]`: A type agnostic lazy iterator object that returns values of type `T` until some end point. Crucially, this does no meaningful work until something calls `Next()` on the iterator.

The interface definition of `Iter[T]` is as follows:
```go
type Iter[T any] interface {
	Next() Option[T]
}
```

### Working with Iterators

#### Producers

todo

#### Transformers

There are two ways of working with iterators as a concept. Each part of code that interacts with an iterator is either a transformer or a consumer. Transformers are functions that take an iterator of type `T` as a parameter and return an iterator of some other type `U`. One can liken this to many internal functions in Teleport. A function like `filterNodes(ctx *AuthContext, nodes []types.Server) []types.Server` could be rewritten using iterators as `filterNodes(ctx *AuthContext, nodes Iter[types.Server]) Iter[types.Server]`. Transformers therefore apply operations like type transforms, filtering and others to the iterator. Because transformers modify the iterator, they themselves don't do any work at the point of the function call, they merely chain logic onto the iterator object that is run when `Next()` is called.

todo

#### Consumers

Consumers are non-lazy functions that take an iterator and consume it to produce something else, i.e they are the final link in the chain that drive the logic contained in the iterator by calling `Next()`. For example, a function in `tsh` might be `printTrackers(ctx context.Context, trackers Iter[types.SessionTracker])`. This function would consume the iterator and for every value it receives, print it to `stdout`.

todo

#### Examples

The examples are using a beta version of [my iterator library](https://github.com/xacrimon/functional).

```go
import "github.com/xacrimon/functional/iter"

type Iter = iter.Iter

// Consumes no amount of intermediary memory to perform the computation, preventing memory spikes that scale with the amount of active sessions.
func FilterTerminatedTrackers(trackers Iter[types.SessionTracker]) Iter[types.SessionTracker] {
	return iter.Filter(trackers, func(tracker *types.SessionTracker) bool {
        return tracker.GetState() != types.SessionStateTerminated
    })
}
```

```go
import (
    "github.com/xacrimon/functional/iter"
    "github.com/xacrimon/functional/result"
)

type Iter = iter.Iter
type Result = result.Result

// Consumes no amount of intermediary memory to perform the computation, preventing memory spikes that scale with the amount of nodes.
func DeserializeNodes(trackers Iter[string]) Iter[Result[types.Server]] {
	return iter.Map(trackers, func(json string) Result[types.Server] {
        item := new(types.ServerV3)
        err := json.Unmarshal([]byte(json), item)
        if err != nil {
            return result.Err(err)
        }

        return result.Ok(item)
    })
}
```

### External API's over gRPC

todo
