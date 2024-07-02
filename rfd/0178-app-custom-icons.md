---
author: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD 178 - Application Resource Custom Icon

## Required Approvers

- Engineering: @romant && @avatus && (someone from the scale team))

## What

Support custom icon uploads for resources through yaml. We can start with feature request [#4712](https://github.com/gravitational/teleport/issues/4712) where
users provide a URL and Teleport's auth server will download the image and store it in the backend.

## Why

Users want to be able to recognize their applications by famaliar icons/logos rather than reading the application name.

Today, the web UI has some hard coded methods to [guess](https://github.com/gravitational/teleport/blob/dc1199636558f85ed275512175a12207a8034d44/web/packages/shared/components/UnifiedResources/shared/viewItemsFactory.ts#L184) which icon to show for an application by looking at the application name (which may or may not be an accurate depiction of the application) and if nothing matches, we use a generic icon to mean `application`. The guess type is limited to a few handpicked applications.

## Details

### User Story

Alice is a system admin and wants to add custom icon for application resources. Alice specifies the icon URL field on the application resource yaml and using `tctl` creates or updates this file.

### Requirements

- Persist icons in a storage
- Efficiently retrieve icons
- Upload icons via yaml resource

### Implementation

#### Persist icons in a storage

Icons can be stored in the backend as blobs with its own row with the key formatted as `icon/<resource-kind>/<name-of-resource>:<uid>`. A unique identifier is required for the proxy cache, otherwise updated icons will not be reflected right away. The UID will be saved in the resource's spec.

#### Efficiently retrieve icons

Currently, in the web UI we fetch 48 resources per listing, that's 48 potential fetch attempts to get the image from the backend from one client. To reduce the amount of trips to the backend, the proxy server will cache the fetched icons in an in-memory sync map that stores icons as byte array with `lastTouched` date to help with cache cleanup.

The maps key will be similar to backend key `<resource-kind>/<name-of-resource>:<uid>`.

Before adding a new icon to cache, we can delete an entry with prefix of `<resource-kind>/<name-of-resource>`, since there will only ever be one icon per resource.

To clean up after possibly deleted icons (eg: deleting an app resource), we can spin a go routine where every day we go through all the values in the map and delete any entries with `lastTouched` date greater than a day.

#### Upload via yaml resource

Users can use the resource spec field `icon` to specify the filepath which Teleport can download the file from.

```proto
message Icon {
  // Start with https://
  string FilePath = 1;
  // This is an internal field, where we set it
  // after we successfully uploaded the icon.
  string UID = 2;
}
```

### Security

Icon upload attempts not meeting the below requirements will be rejected and logged.

#### Check URL Schema

Only allow `https://`

#### Check content type

Check the response header for the correct `content-type` and only accept `image/jpeg`, `image/png`, `image/svg+xml`, and `image/gif`.

Since this can be easily spoofed, we will follow up with golang's http [DetectContentType](https://pkg.go.dev/net/http#DetectContentType) check.

#### Limit size

Read response only up to 100 kb

####

### Future Scope

#### Scale

We could potentially run into scale issues if there are lot of applications with custom icons or if we start allowing uploading custom icons for other resources. If we get to that point, we can offer cloud storage alternatives similar to how we do for [session recordings](https://goteleport.com/docs/reference/backends/).

#### Web UI support for adding icon

On the resource listing page, we could add a `edit` button for each application tile, that allows editing some fields of an application resource.

#### Allow file URI scheme

We could add support to upload image from a `file://` scheme
