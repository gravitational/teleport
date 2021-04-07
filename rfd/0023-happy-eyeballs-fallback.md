---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# RFD 23 - Happy Eyeballs-style default port fallback

## Why

Comments on issue [#4924](https://github.com/gravitational/teleport/issues/4924) 
suggest that `tsh` should use multiple default ports (e.g. 3080 and 443) when a 
proxy port is not specified, and should use a smart fallback algorithm to pick 
a canonical proxy address out of the multiple options provided.

The argument is that the current default port, `3080` is often blocked by 
firewalls but, say, the default HTTPS port `443` is often not. In order to 
improve the UX of users in such a situation, `tsh` should not simply try one
after the other, as the timeouts involved are hostile to the user. Instead, 
`tsh` should employ something like the
["Happy Eyeballs" algorithm](https://tools.ietf.org/html/rfc6555) that will 
try to pick the correct server concurrently.

Given that this requires a change in the network protocol, it probably also 
requires an RFD.

## What

The resolution algorithm boils down to `tsh` firing off multiple near-
simultaneous requests, one to each proxy port in the "default ports" set. The
resolver then races the requests to see who comes back first. The first response
to arrive "wins", and the port that request was sent to becomes the canonical 
port for all further interaction with the proxy to use.

### Which request to race?

#### Option 1: PING
The first interaction that `tsh` has with a proxy is to `ping` it over HTTPS, 
so this would be a good candidate. 

**Good:** Using `PING` means that the roundtrip to the server is not wasted,
and the response can be used immediately once it arrives, reducing the user-
perceived delay.

**Bad:** There is a lot of machinery in the Teleport client that assumes 
that the `ProxyHostAddr` is well-defined _before_ issuing any requests to 
the server. Adding behaviour that defers or changes the `ProxyHostAddr` value 
when host resolution algorithm completes will have to be very careful not to 
trip these up.

#### Option 2: `/`
A second option would be to use a throwaway request for `/`. 

**Good:** This could be inserted _before_ creation of a full Teleport client, 
and reduce the scope for tripping over assumptions about the validity of the 
`ProxyHostAddr`

**Bad:** Introduces another round-trip to the server, increasing the user-
perceived delay.

Given that the whole point of this exercise is to _reduce_ lag felt by the 
user, I'd recommend trying Option 1, even if the implementation is a bit 
more tricky.

### Ancillary changes

1. The client config structure would need to contain a list of alternative 
   addresses (aka `CandidateWebProxyAddrs`) to try during resolution. If a 
   canonical `WebProxyAddr` is not set, `tsh` will invoke the happy eyeballs 
   algorithm to pick a single, canonical `WebProxyAddr` from the list in 
   `CandidateWebProxyAddrs`.

2. The proxy host parser will generate a list of `CandidateWebProxyAddrs`
   if no port is specified in the user-supplied proxy address. Each entry 
   is the user-supplied proxy hostname paired with a port number from the 
   default port set.

3. The teleport client will have to defer construction of things
   that require a canonical `WebProxyAddr` until after it has been 
   determined by the resolution algorithm. (e.g. KeyAgent)