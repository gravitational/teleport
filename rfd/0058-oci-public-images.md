---
authors: Logan Davis (logan.davis@goteleport.com)
state: draft
---

# RFD 58 - Public OCI Images

## **What**

Teleport images are currently hosted on [quay.io](https://quay.io/organization/gravitational). This RFD proposes migrating public images from Quay to [Amazon ECR Public](https://docs.aws.amazon.com/AmazonECR/latest/public/what-is-ecr.html)

## **Why**

As of August 1st, 2021 Quay.io no longer supports any other authentication provider other than Red Hat Single-Sign On.<sup>[[1]]</sup>

## **Details**

### **Alternatives**

#### **Harbor**
Harbor is an open source registry that secures artifacts with policies and role-based access control.<sup>[[2]]</sup> Harbor is a collection of components, the most notable being [Docker Distribution](https://github.com/distribution/distribution), which is an implementation of the [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec). 

Harbor supports authentication via an OIDC single sign-on provider, such as Okta.<sup>[[3]]</sup> 

## References

\[1\] - https://access.redhat.com/articles/5925591
\[2\] - https://goharbor.io/
\[3\] - https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/

[1]: https://access.redhat.com/articles/5925591
[2]: https://goharbor.io/
[3]: https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/