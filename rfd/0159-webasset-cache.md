
---
authors: Michael Myers (michael.myers@goteleport.com)
state: draft
---

# RFD 0159 - Webasset Cache

## Required approvers


## What

We discuss storing the webassets in an s3-compatible storage to allow different versioned proxies
to fetch any file requested that may not already exist in their embedded filesystem.

## Why

Up until now we've always thought of the 1-to-1 relationship of webasset bundle
to proxy version was a given. In most cases, it still is. However, some
users may run multiple versions of the proxy behind a load balancer and this
would cause the web browser to sometimes not be able to load the correct
webasset due to that loadbalanced proxy being of a different verison.

The way the frontend is bundled is by using code splitting. In a non-code split
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
will not have the file and return a `404`. Storing all the webassets in an s3-compatible
bucket allows the proxies to fallback to single repository of every webasset in use.

## Details

### Webasset cache

When the proxy comes online, we can start an (optional) service that will sync the webassets
to a configured, s3-compatible storage. When a proxy comes online, it will start a heartbeat
that first, checks if it's webassets have been loaded. It does this by first, listing the objects in
the bucket using `ListObjectsV2`. As of right now, we have ~170 items in our webassets bundle. 
`ListObjectsV2` will only list up to 1,000 items per page. This means that after 5 stored versions
we will have to start iterating multiple pages to list all the keys. This is unlikely and can be mitigated by
a implementing a decent retention policy (more on that later in the RFD).

Once the keys have been listed, the service can walk through the embedded filesystem to find any files
that have **not been synced** with the bucket and then batch upload them using the s3 sdk [`UploadWithIterator`](https://docs.aws.amazon.com/sdk-for-go/api/service/s3/s3manager/#Uploader.UploadWithIterator)
to cut down on the amount of outgoing network requests. 

There isn't too much of a concern to have key collisions thanks to Vite's rollup hashing. A somewhat recent change
has [moved their hashes to use base64](https://github.com/rollup/rollup/issues/4803) so the chances of the same
split file having the same hash across multiple versions is very low.

#### File cleanup
The first cause for concern is how to mitigate an s3 bucket filling up indefinitely with new versions of the webassets.
Due to the chaotic nature of when/how often different users choose to upgrade, I believe it it out of scope for
Teleport to have any control of bucket cleanup. However, we can provide some general suggestions and documentation
that we think would work best. One of the things Teleport _can_ provide is a `TeleportVersion` tag for each webasset
file (assuming the compatible storage supports tags). This will give the users some insight into which version a specific
file is supporting.

The suggested way to cleanup files is to set a [lifecycle rule](https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lifecycle-mgmt.html) based on expiration. This will
let files naturally cleanup themselves after a specified time. If a file is deleted due to this rule for a proxy version still
in use, then the proxy will just reupload the file during the next heartbeat.

According to AWS, the lifecycle rules are only checked [once per day](https://repost.aws/knowledge-center/s3-lifecycle-rule-delay), and are rounded to midnight UTC of the next day when an object would be "expired".  However, different storages may have different timings here, so it's best the heartbeat runs frequently. (every 10 minutes?)

Ultimately, the user can setup their cleanup however they wish, and Teleport is responsible for filling up the bucket.



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
If it doesn't, we can download the file from the webasset bucket and serve
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
	writer := &MemoryWriterAt{}
	cfg.WebassetHandler.Download(cfg.Context, session.ID(fmt.Sprintf("%s/%s", teleport.Version, fileName)), writer)

	// detect content type
	// compress
	http.ServeContent(w, r, fileNameToSkip, time.Now(), bytes.NewReader(writer.buffer.Bytes()))
}
```


### Security
This feature should be **opt in only** via configuration. 

As of now, only the Auth talks to s3/aws. This means that the proxy would also have to have access to some credentials
that write/read access to the user's chosen storage. This increases the risk surface area by having two susceptible services. 

If someone has gained access to the bucket, they could potentially overwrite files with their own code that the
browser will then try to download. We may be able to get around this by implementing some sort of signing to
our webasset bundles but would not be part of phase1 of this project.

#### Alternative approaches
Another approach that was discussed that didn't involve an s3 bucket was storing all the webassets in an in-memory
cache in the auth server. This would allow any proxy to reach out on start and transfer it's webassets over (if the version
was different) and then any other proxy could reach out to auth to download the missing files.

This had a bit more complexity because it would mean that _all_ auths would have to have _all_ in-use versions of the
webassets to ensure any proxy could hit any auth for the missing files. The benefit of approach is everything would be
self-contained and not rely on external storage. However, being much more complex, with many different listeners/watchers, it could lead to more issues down the road.

#### Cloud
This feature will eventually be integrated by the cloud team as well. More
details to come