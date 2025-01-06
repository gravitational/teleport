---
authors: Michael Myers (michael.myers@goteleport.com)
---

# RFD 179 - Web API backward compatibility guidelines

## What

Discussion around viable options to help version our evolving web API

## Why

The Web API/Client have generally been seen as one-in-the-same with the proxy
service. While that is somewhat true due to the fact that the proxy serves the
web client, there are scenarios where multiple versions of the proxy can exist
in a cluster. This means that the web api should be thought of as a separate
"component" when discussing our [backward compatibility promise](https://goteleport.com/docs/upgrading/overview/#component-compatibility).

## Example

We recently updated a few resources to be paginated in the web UI instead of
sending the entire list. This example is taken from a recent feature update to
the Access Requests list in the Web UI and highlights the issue at hand.

Access Requests in the web UI used a client-side paginated table, and fetched all requests in a single API call.

```go
// example handler
func AccessRequestHandler() ([]AccessRequest, error) {
    accessRequests := []AccessRequest
    return accessRequests,nil
}
```

Some users had too many access requests to send over the fetch and the page
would break, becoming unusable for larger users. The solution was to server
side paginate the Access Requests request. The response used to just be a list
of Access Requests, and it has then changed to an object with a list of
`requests` and `nextKey` as fields.

```go
// example handler
func AccessRequestHandler() ([]AccessRequest, error) {
   accessRequests := []AccessRequest
   nextKey := "hi_im_next"
   return AccessRequestResponse{
       requests: accessRequests,
       nextKey: nextKey,
   }, nil
}
```

This breaks backward compatibility because a web client of version n-1 would be
expecting a list of requests only, and a web client of n hitting a proxy of n-1
would be expecting to receive the paginated list.

The solution _in this particular case_ was

1. on the proxy side, check which version of the web client was making the request (by the existence of a specific query param, `limit`) and decide which shape to send
2. on the web client side, if receiving a response from an old proxy, fallback to the client pagination.

This may work, but it highlights a couple of things:

- we can send any response shape we want with no errors/warnings.
- if backward compatibility wasn't forefront of the mind, there was nothing stopping us from just pushing the new code with a breaking change.
- the backend has to check the request shape and the frontend has to check the response shape just to know what type it is receiving.

Using a standard guideline for our api service/web client should help us evolve our API overtime in a backward compatible way.

### Incrementing version prefix

We should [stop stripping the api prefix](https://github.com/gravitational/teleport/blob/045880f4f5defdea34a70febb82631d7e80ce345/lib/web/apiserver.go#L527-L530), and add a new endpoint with a incremented prefix to accommodate the new response shape.
This will allow us to implement the following guidelines.

```go

// regex to match /v1, /v2, /v3, etc
apiPrefixRegex := regexp.MustCompile(`^/v(\d+)/webapi`)
// request is going to the API?
if matches := apiPrefixRegex.FindStringSubmatch(r.URL.Path); matches != nil {
    version := matches[1]
    // Strip the prefix only if it's v1
    if version == "1" {
        newPath := strings.TrimPrefix(r.URL.Path, "/v1")
        r.URL.Path = newPath
    }
    h.ServeHTTP(w, r)
}

```

We can continue to strip the prefix if the requested path includes `/v1` so we can leave our current response handlers "versionless"
and not have to update the entire api server.

```go
// keep this
h.GET("/webapi/tokens", h.WithAuth(h.getTokens))

// and add this with a new version
h.GET("/v2/webapi/tokens", h.WithAuth(h.getTokensV2))
```

## Web Client/API Backward Compatibility Guidelines

A proxy server version N must support a web client of version N and N-1. A web client of version N _might_ hit a proxy of version
N-1. Follow the guidelines below to understand if a new versioned endpoint is needed to support this edge case.

Please follow these guidelines for common scenarios:

- [Creating a new endpoint](#creating-a-new-endpoint)
- [Removing an endpoint](#removing-an-endpoint)
- [Adding/removing a field to a response of an existing endpoint](#addingremoving-a-field-to-a-response-of-an-existing-endpoint)
- [Updating request body fields](#updating-request-body-fields)
- [New required fields from an API response in the web client](#new-required-fields-from-an-api-response-in-the-web-client)
- [Adding new features to the web client](#adding-new-features-to-the-web-client)

You can also use this checklist to make sure you've covered the backward compatibility promise

### A short checklist when updating the web client/api

- [ ] If a new endpoint was added to accommodate a new feature, the new feature must fail gracefully with the required proxy version - [more details](#adding-new-features-to-the-web-client)
- [ ] When updating a feature, check if the web client works as expected with a proxy version N-1
- [ ] When updating a feature, check that the proxy works receiving requests from web clients N-1
- [ ] If deprecating an endpoint, a TODO is added to delete in the relevant version - [more details](#removing-an-endpoint)
- [ ] When updating an API response, deprecated fields are still populated - [more details](#addingremoving-a-field-to-a-response-of-an-existing-endpoint)

### Creating a new endpoint

Creating a new endpoint that previously didn't exist is ok. This endpoint should
be prefixed with `/v1` if using the REST API paradigm. The response shape should
always return an object, even if that object would contain only one field.
Prefer pagination for resources that can be paginated, and always create the
response as if they _could_ be paginated in the future.

Bad

```go
func (h *Handler) getFoos() (interface{}, error) {
    foos := []string{"foo1", "foo2", "foo3"}
	return foos, nil
}
```

Good

```go
type GetFoosResponse struct {
    Foos []string
    NextKey string
    ContainsDuplicates bool
}

func (h *Handler) getFoos() (interface{}, error) {
    foos := []string{"foo1", "foo2", "foo3"}
	return GetFoosResponse{
	     Foos: foos,
	     ContainsDuplicates: true
	}, nil
}
```

#### Defining new endpoints in the web client

All endpoints should be defined in their own objects inside the `cfg.api` object in appropriate project's `config.ts` file, like in [this example](https://github.com/gravitational/teleport.e/blob/ebf079267020d59f14353dd3495b4fd783339fa5/web/teleport/src/config.ts#L134).

This helps us tell apart same paths but with different HTTP verbs.

Example of a new single endpoint:

```ts
user: {
  create: '/v1/webapi/users',
}
```

Example of an endpoint with same paths but with different verbs:

```ts
user: {
  create: '/v1/webapi/users',
  update: '/v1/webapi/users',
}
```

Example of creating a version N endpoint:

```ts
// Note: only mark the old endpoint for deletion later if we need to
// handle a fallback, otherwise we can delete the old endpoint at the
// introduction of a v2 endpoint.
user: {
  // TODO(<your-github-handle>): DELETE IN 18.0 - replaced by /v2/webapi/users
  create: '/v1/webapi/users',
  createV2: '/v2/webapi/users',
}
```

### Removing an endpoint

An endpoint can be removed in a major version n+2, where n is the last major
version where the endpoint was used.

Mark endpoints that needs to be removed with:

```go
// TODO(<your-github-handle>): DELETE IN 18.0
h.GET("/webapi/tokens", h.WithAuth(h.getTokens))
```

Example 1: v17 no longer uses GET /webapi/foos which was last used in v16. The
endpoint can be removed in v18.

Example 2: v4.2.0 no longer uses GET /webapi/bars which was last used in v4.1.3. The endpoint can be removed in v6, so that v5 still supports clients using v4.1.3 and v4.2.0.

### Adding/removing a field to a response of an existing endpoint

Adding a new field to a response is OK as long as that field has no effect on
the previously existing fields. Any field that was previously in a response
should _stay populated_ in the new response, even if that creates duplicate
data. An existing field _cannot_ have its type changed. A field should not be
removed until the backward compatibility promise has been 'fulfilled'. Follow
[removing an endpoint](#removing-an-endpoint) guidelines for removing fields.

Bad

```go
type GetFoosResponse struct {
    Foos []string
    NextKey string
    // ContainsDuplicates bool <--- previously existing field made redundant by DuplicateCount
    DuplicateCount int
}

func (h *Handler) getFoos() (interface{}, error) {
    foos := []string{"foo1", "foo2", "foo3", "foo3"}
	return GetFoosResponse{
	     Foos: foos,
         // ContainsDuplicates must be populated to support older clients.
	     DuplicateCount: 1
	}, nil
}
```

Good

```go
type GetFoosResponse struct {
    Foos []string
    NextKey string
    // TODO(avatus): DELETE IN 18.
    // Deprecated: Use `DuplicateCount` instead.
    ContainsDuplicates bool
    DuplicateCount int
}

func (h *Handler) getFoos() (interface{}, error) {
    foos := []string{"foo1", "foo2", "foo3", "foo3"}
	return GetFoosResponse{
	     Foos: foos,
         // ContainsDuplicates must be populated to support older clients.
	     ContainsDuplicates: true
	     DuplicateCount: 1
	}, nil
}
```

If a response shape must change entirely, prefer creating a new endpoint

### Updating request body fields

By default, prefer a new versioned endpoint when updating the request shape. For
example, you have an API `listFoos(startKey: string)` and you add a new request
param `connectedOnly: bool` to it. If you hit a proxy of n-1, the response will
still be ok, just the new param will be silently dropped and you will get all
"foos".

A Request body can be updated and reuse the current endpoint with a new field as
long as the omission of the new field would not cause a breaking change (i.e.,
an old client sending the old request without the new field). If the handler
_requires_ this new field in order to function, then a new endpoint must be
created with a version increment.

The existing request adding in the new `MyNewThing` field

```go
type MyRequest struct {
    Limit int
    Query string
    MyNewThing string // <-- new field
}
```

if the handler has code along the lines of this

```go
if req.MyNewThing == "" {
     return nil, err("BAD REQUEST")
}
```

then that is a _BREAKING CHANGE_ and should thus have a new versioned handler.

### Adding new features to the web client

If a feature requires a new endpoint, it must properly handle the case of a 404.
This means making the "expected" 404 error clear to the user that an endpoint is
not found due to a version mismatch (and preferably which minimum version is
needed) rather than an ambiguous "Not Found" error. This works mostly for a
"resources" endpoint where it is expected to return an empty list if something
isn't found (i.e., "no access requests"), but a new endpoint that returns a
single resource _should not_ assume that a 404 means anything other than a
resource is not found. In this particular case, there isn't much graceful work
we can do here. Graceful handling of this error state should be best effort.

### New required fields from an API response in the web client

If the updated feature _cannot_ function without receiving new fields from the
API (for example, receiving a response from a proxy version N-1), refer to the
API guidelines about [creating a new versioned endpoint](#creating-a-new-endpoint). If the feature itself is degraded
but still usable (be conservative with your discretion on this/ask product), a
warning alert should be shown to the user with information about

- What part of the feature may not work.
- What version their proxy must be updated to in order to be fully operational.

An example of this in recent memory is Pinned Resources. The "pinned resources"
tab was available in clients that _may_ have been communicating with proxies
older than the required version. So a check was made to see if that proxy
supported pinned resources, and if it wasn't, made the error known to the user
that "Pinned Resources is supported by proxies version 14.1+" (or something like
that).
