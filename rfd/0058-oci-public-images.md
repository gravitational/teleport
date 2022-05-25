---
authors: Logan Davis (logan.davis@goteleport.com)
state: draft
---

# RFD 58 - Public OCI Images

## **What**

Teleport images are currently hosted on [quay.io](https://quay.io/organization/gravitational). This RFD proposes migrating public images from Quay to [Amazon ECR Public](https://docs.aws.amazon.com/AmazonECR/latest/public/what-is-ecr.html)

## **Why**

As of August 1st, 2021 Quay.io no longer supports any other authentication provider other than Red Hat Single-Sign On.<sup>[[1]]</sup> Moving images from Quay.io to Amazon ECR Public has an additional benefit of further consolidating infrastructure to AWS.

## **Details**

Moving our public image infrastructure from [Quay.io](https://quay.io/) to Amazon [Elastic Container Registry](https://aws.amazon.com/ecr/) gives Teleport improved security controls with Amazon SSO + Okta, as well as IAM policies + Security Auditing. In addition to the aforementioned improvements, [image tag mutability](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-tag-mutability.html) can prevent accidental overwrites and add an additional layer of security to ensure images are not replaced maliciously. 

### Infrastructure
Hosting our own _oci-compatible_ registry is similar to hosting our own [terraform registry](https://github.com/gravitational/teleport-plugins/blob/master/rfd/0002-custom-terraform-registry.md). However, the [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec) has additional complexities that can't be solved by S3 and CloudFront<sup>*</sup> alone. These complexities warrant the use of [Docker Registry](https://docs.docker.com/registry/).

```
                                        oci.releases.teleport.dev

                                                    │
                                                    │
                                                    ▼
                                            ┌──────────────┐
                                            │  CloudFront  │
                                            └──────────────┘

                                                    │
                                                    ▼
                                          ┌───────────────────┐
                                          │  Docker Registry  │
                                          └───────────────────┘

                                                    │
                                                    ▼
                                                  ┌────┐
                                                  │ S3 │
                                                  └────┘
```

Note<sup>*</sup>: A minimal, read-only, _oci-compatible_, registry could be mimicked through CloudFront functions. See alternatives(TODO: Add link to this alternative)

### **Alternatives**

#### **Harbor**
Harbor is an open source registry that secures artifacts with policies and role-based access control.<sup>[[2]]</sup> Harbor is a collection of components, the most notable being [Docker Distribution](https://github.com/distribution/distribution), which is an implementation of the [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec). 

Harbor can be installed via [Docker Compose](https://docs.docker.com/compose/) on an EC2 instance or via Helm directly onto a kubernetes cluster for HA. 

Harbor supports authentication via an OIDC single sign-on provider, such as Okta.<sup>[[3]]</sup> 

## References

\[1\] - https://access.redhat.com/articles/5925591
\[2\] - https://goharbor.io/
\[3\] - https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/

[1]: https://access.redhat.com/articles/5925591
[2]: https://goharbor.io/
[3]: https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/

## Brain Dump

* CloudFront needs rate limiting to prevent clients from running up too high of a cost
* CloudFront with javascript functions to mimic the read part of API
* CloudFront to EC2 with some type of docker registry process
* Image signing with Cosign
* Images stored in `aws-teleport-team-prod`
* Will immutable tags allow for better caching? Since image should never change 

* Multi step process. AWS ECR Infrastructure in the `cloud-terraform` repository
* Push to Quay and ECR
* Replicate existing images from quay.io to ECR
* update documentation to new location 
* retire quay repository
