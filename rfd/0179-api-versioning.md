
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
---

# RFD 179 - Web API backward compatibility and typing system

## What

Discussion around viable options to help version and type our evolving web API

## Why

The Web API/Client have generally been seen as one-in-the-same with the proxy service. While that is somewhat true due to the fact that the proxy serves the web client, there are scenarios where multiple versions of the proxy can exist in a cluster. This means that the web api should be thought of as a separate "component" when discussing our [backward compatibility promise](https://goteleport.com/docs/upgrading/overview/#component-compatibility).  Also, the types for our api are completely opaque between the client code and the server code. This leads to guessing json responses, unnecessary type conversions on the client and server, and lack of guardrails around the changing of request/response types. It is not only bad ergonomics for developers, but leaves the door wide open for breaking backward compatibility.   

## Example
We recently updated a few resources to be paginated in the web UI instead of sending the entire list. This example is taken from a recent feature update to the Access Requests list in the Web UI and highlights the issue at hand.

Access Requests in the web UI used a client side paginated table, and would fetch every request in the initial API call.
```go
// example handler
func AccessRequestHandler() ([]AccessRequest, error) {
    accessRequests := []AccessRequest
    return accessRequests,nil
}
```
 Some users had too many access requests to send over the fetch and the page would break, becoming unusable for larger users. The solution was to server side paginate the Access Requests request. The return used to just be a list of Access Requests, and it has then changed to an object with a list of Access Requests as a field and a nextKey field. 
 
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

This breaks backward compatibility because a web client of version n-1 would be expecting a list of requests only, and a web client of n hitting a proxy of n-1 would be expecting to receive the paginated list.

The solution _in this particular case_ was
1. on the proxy side, check which version of the web client was making the request (by the existence of a specific query param, `limit`) and decide which shape to send
2. on the web client side, if receiving a response from an old proxy, fallback to the client pagination.

This may work, but it highlights a couple things.
- we can send any response shape we want with no errors/warnings. 
- if backward compatibility wasn't forefront of the mind, there was nothing stopping us from just pushing the new code with a break
- the backend has to check the request shape and the frontend has to check the response shape just to know whats happening.

## Solution

**Summary**: Leverage ConnectRPC and create a web service that can use a proto schema to define our requests and response types. Also, optionally, use a modern fetching framework to simplify and unify the way we make api requests in the web client. This will remove boilerplate that we need to use our APIs in the web client, provide type safety out of the box for both server and client code, and provide backward compatibility guardrails when creating/updating our APIs. 


### Creating a new rpc service with Connect
(buf's Connectrpc, not Teleport Connect)

Connect is a library used in making gRPC compatible HTTP apis. It supports [multiple protocols](https://connectrpc.com/docs/introduction#seamless-multi-protocol-support)  and can generate go service code and typescript client code from the proto schema. We do something similar in Teleport Connect where we generate a client for `tsh` to make requests to the server and use the generated typescript code. This would be doing the same for the web client now.

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
  // The next_page_token value returned from a previous List request, if any.
  string page_token = 2;  
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
import  { createPromiseClient }  from  "@connectrpc/connect";  
import  { createConnectTransport }  from  "@connectrpc/connect-web";  
  
// Import service definition that you want to connect to.  
import  {  WebService  }  from  "@buf/connectrpc_web.connectrpc_es/connectrpc/web/v1/web_connect";  
  
// The transport defines what type of endpoint we're hitting.  
// In our example we'll be communicating with a Connect endpoint.  
// If your endpoint only supports gRPC-web, make sure to use  
// `createGrpcWebTransport` instead.  
const transport =  createConnectTransport({  
    baseUrl:  "https://demo.connectrpc.com",  
});  
  
// Here we make the client itself, combining the service  
// definition with the transport.  
const client =  createPromiseClient(WebService, transport);


// and then used in the react code simply (a bit contrived)
<form  onSubmit={async  (e)  =>  {  
    e.preventDefault();  
    await client.listFoos({  
        pageSize,
        nextKey  
    });  
}}>  
    <input  value={pageSize}  onChange={e =>  setPageSize(e.target.value)}  />  
    <button  type="submit">Send</button>  
</form>
```



Using proto messages to define our request and response shapes will give type safety in the server and client automatically, and keep it consistent between the two. As of now, the web client receives any arbitrary json response and we must coerce it into a [type for every different resource.](https://github.com/gravitational/teleport/blob/master/web/packages/teleport/src/services/joinToken/makeJoinToken.ts#L25) . Making a change in the backend is not reflected in the client currently and can lead to bugs, especially if the backend is worked on independently. Using proto definitions here would mean the client would break and can be caught in tests before the change is pushed to master.

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
````
because the client is already set up, we don't need to fiddle with urls, query params, untyped json responses, service wrappers. We would still need to `useAsync` for the client call unless...

#### Tanstack Query
One of the most popular query libraries in the last few years is Tanstack Query (formerly known as React Query). Connectrpc has an o[fficial tanstack query plugin](https://connectrpc.com/docs/web/query/getting-started) which removes the need to use `useAsync`. Although it may seem like "just a 4th way to fetch requests" in the web, if we use it from the start with the new Connect client, it can set the standard going forward.

```typescript
  const { isPending, error, data } = useQuery(listFoos)
```

[Check out the docs for Tanstack Query](https://tanstack.com/query/latest/docs/framework/react/overview)


## Security
Connect recommends that authentication of api requests to [happen in `net/http` middleware](https://connectrpc.com/docs/governance/rfc/go-cors-authn/#authentication) . Since this is already written for our current http API, we can reuse existing code. (Maybe even take a look at the [cookie/bearer token issue](https://github.com/gravitational/teleport/blob/master/lib/web/apiserver.go#L4616-L4628) at the same time?).

Prehog already uses Connect so we don't need to add a new dependency. If we decided to use Tanstack Query as our fetch library in the frontend, we would have to add it as a web dependency but the library is [quite heavy used/maintained](https://github.com/tanstack/query) and has an MIT license. (We also use this in TAG).

## Alternatives
### Incrementing version prefix
A simple solution would be to [stop stripping the api prefix](https://github.com/gravitational/teleport/blob/master/lib/web/apiserver.go#L527-L530), and add a new endpoint with a incremented prefix to accommodate the new response shape. 
```go
		// regex to match /v1, /v2, /v3, etc.
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
			return
		}
```

This only helps with half of the issue. It allows newer webclients to call
updated endpoints if needed, and preserves the functionality of the old endpoint
for older clients. However, this still doesn't provide anything for type safety
in the server or the clients. And really is still up to the developer/reviewers
to make sure this pattern is followed correctly.