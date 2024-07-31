
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
---

# RFD 179 - Web API backward compatibility and typing system

## What

Discussion around viable options to help version and type our evolving web API

## Why

The Web API/Client have generally been seen as one-in-the-same with the proxy
service. While that is somewhat true due to the fact that the proxy serves the
web client, there are scenarios where multiple versions of the proxy can exist
in a cluster. This means that the web api should be thought of as a separate
"component" when discussing our [backward compatibility promise](https://goteleport.com/docs/upgrading/overview/#component-compatibility).
Also, the types for our api are completely opaque between the client code and
the server code. This leads to guessing json responses, unnecessary type
conversions on the client and server, and lack of guardrails around the changing
of request/response types. It is not only bad ergonomics for developers, but
leaves the door wide open for breaking backward compatibility.  

## Example
We recently updated a few resources to be paginated in the web UI instead of
sending the entire list. This example is taken from a recent feature update to
the Access Requests list in the Web UI and highlights the issue at hand.

Access Requests in the web UI used a client side paginated table, and would fetch every request in the initial API call.
```go
// example handler
func AccessRequestHandler() ([]AccessRequest, error) {
    accessRequests := []AccessRequest
    return accessRequests,nil
}
```
 Some users had too many access requests to send over the fetch and the page
 would break, becoming unusable for larger users. The solution was to server
 side paginate the Access Requests request. The return used to just be a list of
 Access Requests, and it has then changed to an object with a list of Access
 Requests as a field and a nextKey field.
 
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

This may work, but it highlights a couple things.
- we can send any response shape we want with no errors/warnings. 
- if backward compatibility wasn't forefront of the mind, there was nothing stopping us from just pushing the new code with a break
- the backend has to check the request shape and the frontend has to check the response shape just to know what type it is receiving.

Using a standard guideline for an rpc service should help us evolve our API overtime in a backward compatible way.

## Web Client/API Backward Compatibility Guidelines

In general, if the Request or Response shape does not change, logic in an API handler can change without worry of backward compatibility.

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

### Removing an endpoint
An endpoint can be removed after our backward compatibility promise has been
fulfilled and NOT before. This means marking for deletion with a TODO and
removing it in the relevant version. For example, if v17 no longer uses
`GET /webapi/foos` , you can mark for deletion in v18, as v18 only needs to
support n-1 versioned clients. It is the responsibility of the developer who is
"removing the use" (i.e., making it obsolete with some other endpoint) to mark
the TODO.


### Adding/removing a field to a response of an existing endpoint
Adding a new field to a response is OK as long as that field has to effect on
the previously existing fields. Any field that was previously in a response
should _stay populated_ in the new response, even if that creates duplicate
data. An existing field _cannot_ have its type changed. A field should not be
removed until the backward compatibility promise has been 'fulfilled'. If v17
clients are only using a new field, you can mark the old field as "TODO DELETE"
in v18. This means that v17 servers can support v16 clients (still using the old
field) and v18 only needs to support v17 (using the new field).

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
	     // somedev: "we don't need ContainsDuplcates anymore because now we can
	     // just check if duplicate count > 0!"
	     DuplicateCount: 1 
	}, nil
}
```

Good
```go
type GetFoosResponse struct {
    Foos []string
    NextKey string
    // TODO (avatus) DELETE IN 18
    ContainsDuplicates bool
    DuplicateCount int
}

func (h *Handler) getFoos() (interface{}, error) {
    foos := []string{"foo1", "foo2", "foo3", "foo3"}
	return GetFoosResponse{
	     Foos: foos,
	     // somedev: "we must keep ContainsDuplicates populated to support older clients!"
	     ContainsDuplicates: true
	     DuplicateCount: 1 
	}, nil
}
``` 
If using ConnectRPC (a proposed solution later in this RFD), then we must _not_ remove fields from requests/responses, even if unused. Mark as deprecated and move on.

If a response shape must change entirely, prefer creating a new endpoint/RPC.

### Updating Request body fields
A Request body can receive a new field as long as the omission of the new field
would not cause a break (i.e., an old client sending the old request without the
new field). If the handler _requires_ this new field in order to function, then
a new endpoint must be created with a version increment.


### Adding new features to the web client
If a feature requires a new endpoint, it must properly handle the case of a 404.
This means making the "expected" 404 error clear to the user that an endpoint is
not found due to a version mismatch (and preferably which minimum version is
needed) rather than an ambiguous "Not Found" error.

### New required fields from an API response in the web client
If the updated feature _cannot_ function without receiving new fields from the
API (for example, receiving a response from a proxy version N-1), refer to the
API guidelines about creating a new endpoint. If the feature itself is degraded
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

### Updating request fields in the web client
Updating what fields are sent in a request body is ok as long as the API follows the updated request fields guidelines. 

---
### A short checklist when updating the web client/api
- [ ] If a new endpoint was added to accommodate a new feature, the new feature must fail gracefully with the required proxy version
- [ ] When updating a feature, check if the web client works as expected with a proxy version N-1
- [ ] When updating a feature, check that the proxy works receiving requests from web clients N-1
- [ ] If deprecating an endpoint, a TODO is added to delete in the relevant version
- [ ] When updating an API response, deprecated fields are still populated

---

## Solution

#### Incrementing version prefix
We should [stop stripping the api prefix](https://github.com/gravitational/teleport/blob/master/lib/web/apiserver.go#L527-L530), and add a new endpoint with a incremented prefix to accommodate the new response shape.
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

This would allow us to follow the guidelines above when it is necessary to
change required request/response shapes. This solves the problem of fixing
backward compatible changes and is 'good enough'. But if we are in the mode of
updating our DX, I propose updating our web client/server communication all
together with ConnectRPC to not only receive help with backward compatibly with
protomessages, but also give us type safety in the server/client code
automatically.

### Add a new RPC service for the web API

**Summary**: Leverage ConnectRPC and create a web service that can use a proto
schema to define our requests and response types. Also, optionally, use a modern
fetching framework to simplify and unify the way we make api requests in the web
client. This will remove boilerplate that we need to use our APIs in the web
client, provide type safety out of the box for both server and client code, and
provide backward compatibility guardrails when creating/updating our APIs.


### Creating a new rpc service with Connect
(buf's Connectrpc, not Teleport Connect)

Connect is a library used in making gRPC compatible HTTP apis. It supports
[multiple protocols](https://connectrpc.com/docs/introduction#seamless-multi-protocol-support)
and can generate go service code and typescript client code from the proto
schema. We do something similar in Teleport Connect where we generate a client
for `tsh` to make requests to the server and use the generated typescript code.
This would be doing the same for the web client now.

Example service (taken from [RFD 153](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md))
```proto
syntax = "proto3";  
  
service WebService {  
rpc ListFoos(ListFoosRequest) returns (ListFoosResponse) {}  
}  
  
message ListFoosRequest {  
  // The maximum number of items to return.
  // The server may impose a different page size at its discretion.
  int32 page_size = 1;
  // The next_token value returned from a previous List request, if any.
  string next_key = 2;  
}  
  
message ListFoosResponse {  
  // The page of Foo that matched the request.
  repeated Foo foos = 1;
  // Token to retrieve the next page of results, or empty if there are no
  // more results in the list.
  string next_page_token = 2;
}
```
Would output generated code for the server and client, and could be used in the client like this
```typescript
// and then used in the react code simply (a bit contrived)
<form onSubmit={async(e) => {  
    const response = await client.listFoos({  
        pageSize,
        nextKey
    });  
}}>  
    <input value={pageSize} onChange={e =>  setPageSize(e.target.value)} />
    <button type="submit">Send</button>
</form>
```

Using proto messages to define our request and response shapes will give type
safety in the server and client automatically, and keep it consistent between
the two. As of now, the web client receives any arbitrary json response and we
must coerce it into a
[type for every different resource.](https://github.com/gravitational/teleport/blob/master/web/packages/teleport/src/services/joinToken/makeJoinToken.ts#L25)
. Making a change in the backend is not reflected in the client currently and
can lead to bugs, especially if the backend is worked on independently. Using
proto definitions here would mean the client would break and can be caught in
tests before the change is pushed to master.

We also get the benefit of backward/forward compatibility that protobufs provide. 
Connect uses HTTP under the hood to make its requests, which makes it easily inspectable in the network tab, and are still `curl` able much like our current API.  

#### Improvements to our fetching process
The web currently uses three different fetching "libraries" (written by us). `useAttempt`, `useAttemptNext`, and `useAsync`. The most recent one is `useAsync` and is quite complete, but it comes with the baggage of our `fetch` wrapper and the many layers of our api service. Here is an example of what needs to happen when making a new api request.

- First, we need to define the endpoint in `config.ts`and also make a "url builder" to make query parameters.
```typescript
const config = {
//...
    joinTokensPath: '/v1/webapi/tokens',
//..
}

getUnifiedResourcesUrl(clusterId: string, params: UrlResourcesParams) {
  return generateResourcePath(cfg.api.unifiedResourcesPath, {
    clusterId,
    ...params,
  });
},
```

- Then each service defines its http calls 
```typescript
upsertJoinToken(req: JoinTokenRequest): Promise<JoinToken> {
  return api
    .put(cfg.getJoinTokenYamlUrl(), {
      content: req.content,
    })
    .then(makeJoinToken);
}
```

- THEN we need to wrap this call in `useAsync` and pass in the `makeXResource` method to get the json
response into a type usable by the UI
```typescript
 const [deleteTokenAttempt, runDeleteTokenAttempt] = useAsync(async () => ctx.joinTokenService.deleteJoinToken(token))
 ```
 
and now we have access to the attempt's status (process, error, etc), 

Using the Connect generate code and the (one time setup client), all of that would boil down to
```typescript
await client.listFoos({ pageSize, nextKey });
```
because the client is already set up, we don't need to fiddle with urls, query
params, untyped json responses, service wrappers. We would still need to
`useAsync` for the client call unless...

#### Tanstack Query
One of the most popular query libraries in the last few years is Tanstack Query
(formerly known as React Query). Connectrpc has an
o[fficial tanstack query plugin](https://connectrpc.com/docs/web/query/getting-started)
which removes the need to use `useAsync`. Although it may seem like "just a 4th
way to fetch requests" in the web, if we use it from the start with the new
Connect client, it can set the standard going forward.

```typescript
  const { isPending, error, data } = useQuery(listFoos)
```

[Check out the docs for Tanstack Query](https://tanstack.com/query/latest/docs/framework/react/overview)


## Security
Connect recommends that authentication of api requests to
[happen in `net/http` middleware](https://connectrpc.com/docs/governance/rfc/go-cors-authn/#authentication)
. Since this is already written for our current http API, we can reuse existing
code. (Maybe even take a look at the
[cookie/bearer token issue](https://github.com/gravitational/teleport/blob/master/lib/web/apiserver.go#L4616-L4628)
at the same time?).

Prehog already uses Connect so we don't need to add a new dependency. If we
decided to use Tanstack Query as our fetch library in the frontend, we would
have to add it as a web dependency but the library is
[quite heavy used/maintained](https://github.com/tanstack/query) and has an MIT
license. (We also use this in TAG).

## Forward thinking
Idealy we could switch over most of the api slowly to use this new service
rather than use the old way for everything that wouldn't possibly be changed
(like login and what not) but that is an understandably big task. Starting with
new services and services that get updated/maintained should be first steps and
a decision can be made later to retrofit the api.