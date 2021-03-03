---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: draft
---

# RFD 20 - Automatic rotation of CAs about to expire

## What

The default CA TTL should be decreased or made configurable.  Since it may
not remain feasible to trigger CA rotation manually under such circumstances,
a mechanism ensuring automatic rotation of CAs that are about to expire should
also be introduced.

## Why

The lifetime of all the CA certificates employed by Teleport is controlled by
the `defaults.CATTL` constant whose value is currently set to 10 years.
If such a long-lived key pair were obtained by a third party, they would be
able to execute MITM or impersonation attacks for prolonged periods of time.

A natural mitigation would be to lower the default CA TTL to a shorter interval
(e.g., 1 year).  Alternatively the CA TTL values could be exposed via
configuration options so that each user can determine the risk/convenience
trade-off appropriate for their needs.

In either case, it may not remain feasible to rely on the single CA rotation
mechanism currently available, which is to explicitly trigger the rotation
using `tctl auth rotate`.

## Details

Taking advantage of the periodic operations already implemented in Teleport, it
would be easy to introduce a check that periodically detects whether a CA is
about to expire.  When the condition obtains, the CA will be auto-rotated with
the default rotation grace period (`defaults.RotationGracePeriod`) of 30 hours.

### The condition of "about to expire"

A CA is said to be "about to expire" if the following condition is found to be
satisfied:
```go
time.Now() >= CA.NotAfter.Sub(CA.AboutToExpirePeriod)
```

With an eye towards configurable CA TTLs, the definition of a CA's
`AboutToExpirePeriod` is based on the TTL value of that CA:
```go
CA.AboutToExpirePeriod = CA.TTL / 12
```

With a CA TTL of 1 year, `AboutToExpirePeriod` would correspond to 1 month.

Note that CA TTLs must be guaranteed to be longer than 15 days to avoid the
case of discovering a CA about to expire too late and ending up without
a valid CA certificate – even if just for a short time, in the better case –
since `15 days / 12 = 30 hours`.
