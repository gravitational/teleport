
---
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
---

# RFD 0159 - Webasset Cache

## Required approvers


## What

We discuss the creation of an in-memory cache of webassets in the auth server to
provide a way for proxies to serve files from different versions of Teleport.
Here is a quick PoC https://www.loom.com/share/9983cae0e8574149a92dcec623e07f10

## Why

Up until now we've always thought of the 1-to-1 relationship of webasset bundle
to proxy version was a given. In most cases, it still is. However, some
customers may run multiple versions of the proxy behind a load balancer and this
would cause the web browser to sometimes not be able to load the correct
webasset.

The way the frontend is bundle is by using code splitting. In a non-code split
app, the entire react bundle would be compiled into a single javascript file and
served with the index.html. This leads to some pretty large page loads so the
solution is code splitting, which automatically splits the javascript bundle
into bite-sized (byte-sized 🤭) chunks that can be served only when needed (page
load, component load, etc etc). These chunks are generally split by feature and
the files are appended with hashes that change every build (every version). If a
web client receives an index.html that will request a file such as
`index-123123.js` and then later on during the session requests
`index-234234.js`, if the proxy that is hit when load balancing is of a
different version than the original that served the `index.html`, that proxy
will not have the file and return a `404`. Storing all the webassets in a cache
on auth will allow the proxies to fallback to auth as a single source of truth
and serve any file needed.

## Details

### Webasset cache

The webasset cache is a simple `map[string][]byte` that lives in the auth server.


```go
type WebassetCache struct {
	webassets map[string][]byte
}
```

This cache gets populated when a proxy comes online. The proxy will initialize
and register and then emit a message for each one of it's webasset files. This
includes everything in the embedded filesystem such as javascript, css, fonts,
images, etc. The total size, uncompressed, of the bundles are around ~11mb for
enterprise and slightly less for oss, but for the sake of discussion we can
average it at around 10mb. This means the cache should be roughly 10mb per
version stored. As of now, we only know of customers running two versions
simultaneously.

A simple example of the new grpc service would look like this

```protobuf
message GetWebassetRequest {
  string name = 1;
}

message GetWebassetResponse {
  string name = 1;
  bytes content = 2;
}

service WebassetCacheService {
  // GetWebasset
  rpc GetWebasset(GetWebassetRequest) returns (GetWebassetResponse);
}
```

Depending how we end up syncing the webassets to each available auth, we may
also add another message of `SendWebasset` along the same lines as Get above.

#### Syncing webassets to auth

Because there can be multiple auth servers in a cluster, each proxy will need to
report it's assets to every auth server in the cluster. We can do this by
INSERT_HOW_TO_DO_THIS. 
The proxy should listen to auth heartbeats and be able to ask "have I sent this file
to this auth recently" and if not, send it over. These will be stored by fileName->fileBytes. If a file
exists in the cache already when it's sent (two of the same version), we can
check the bytes to see if they match. If they don't match, we should remove the
file from the cache as we don't know which proxy is sending the correct file.
Worst case scenario, the file isn't found and the proxy returns a `404` similar
to what it does now. Best case scenario, we prevented some mismatch 
file that would break code, or perhaps a malicious file from being stored in the cache. 
We can store removed fileNames in a separate list to ensure a removed file isn't just 
added again later after removal.

#### Alternatives to syncing with auth
An alternative that was suggested to store the web assets reported to auth in
the backend. That way, if auth didn't have the asset the proxy was asking for
(in a multi auth situation) then the backend could be the final fallback but
some of the webassets are too large to store in the backend per item (400kb for
dynamoDB for example). So we scrapped this idea.


#### Serving the missing webassets
Currently, our static file serving looks [like this](https://github.com/gravitational/teleport/blob/master/lib/web/apiserver.go#L513C4-L518)
```go
if strings.HasPrefix(r.URL.Path, "/web/app") {
    fs := http.FileServer(cfg.StaticFS)

    fs = makeGzipHandler(fs)
    fs = makeCacheHandler(fs)

    http.StripPrefix("/web", fs).ServeHTTP(w, r)
}
```

nothing too complicated besides setting some headers, compression, and then
serving whatever file is requested. The change would be instead of serving,
first check if the file exists in the embedded file system. If it does, send as usual.
If it doesn't, we can fetch the requested file over gRPC and serve the file bytes 
instead with something like this
```go
if strings.HasPrefix(r.URL.Path, "/web/app") {
	// do everything the same if the file exists
	if (fileExists) {
		fs := http.FileServer(cfg.StaticFS)

		fs = makeGzipHandler(fs)
		fs = makeCacheHandler(fs)

		http.StripPrefix("/web", fs).ServeHTTP(w, r)
        return
	}
    res, err := h.auth.proxyClient.GetWebasset(cfg.Context, &webassetcache.GetWebassetRequest{
		Name: r.URL.Path[len("/web/app/"):],
	})
	// detect content type
	// compress
	// store the file in this proxy's webasset cache to prevent future requests
	http.ServeContent(w, r, "example.txt", time.Now(), compressedFileHere)
}
```


### Security Concerns
We aren't serving any files directly from a file system and only fetching from 
a map so there shouldn't be any worry about retrieving files that aren't in the map.
Additionally, only the internal proxy role should be able to set/get these files
in the map and if a proxy has been compromised, this webasset cache is the least
of our concerns.

This feature should be **opt in only** via configuration. 

Another thing to consider is not allowing this cache to blow up and run OOM. I
don't currently see any non-arbitrary solution to this. For example, we could
store the webassets by _version_ rather than just file and limit it, for
example, to two versions. But then what happens if a customer wants to run 3
versions? How many versions do we allow? I think making this an explicit choice
by the customer to turn on should prevent any sort of "accidental" misuse enough
for now.