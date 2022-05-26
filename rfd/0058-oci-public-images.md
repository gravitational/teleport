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

### **Infrastructure**
Hosting our own _oci-compatible_ registry is similar to hosting our own [terraform registry](https://github.com/gravitational/teleport-plugins/blob/master/rfd/0002-custom-terraform-registry.md). However, the [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec) has additional complexities that can't be solved by S3 and CloudFront<sup>*</sup> alone. These complexities warrant the use of [Docker Registry](https://docs.docker.com/registry/).

An example infrastructure diagram is shown below:

```
                                    ┌───────────────────────────────────┐
                                    │                                   │
                                    │     oci.releases.teleport.dev     │
                                    │                                   │
                                    │                 │                 │
                                    │                 │                 │
                                    │                 ▼                 │
                                    │   ┌──────┬──────────────┬──────┐  │
                                    │   │      │  CloudFront  │      │  │
                                    │   │      └──────────────┘      │  │
                                    │   │                            │  │
                                    │   │             │              │  │
                                    │   │             │              │  │
                                    │   │             ▼              │  │
                                    │   │  ┌───────────────────────┐ │  │
                                    │   │  │ Elastic Load Balancer │ │  │
                                    │   │  └───────────────────────┘ │  │
                                    │   │                            │  │
                                    │   │             │              │  │
                                    │   │             │              │  │
                                    │   │             ▼              │  │
                                    │   │  ┌───────────────────────┐ │  │
                                    │   │  │ ECS / EKS / EC2       │ │  │
                                    │   │  │ ┌───────────────────┐ │ │  │
                                    │   │  │ │  Docker Registry  │ │ │  │
                                    │   │  │ └───────────────────┘ │ │  │
                                    │   │  │                       │ │  │
                                    │   │  └───────────────────────┘ │  │
                                    │   │                            │  │
                                    │   │             │              │  │
                                    │   │             │              │  │
                                    │   │             ▼              │  │
                                    │   │          ┌────┐            │  │
                                    │   │          │ S3 │            │  │
                                    │   │          └────┘            │  │
                                    │   │WAF                         │  │
                                    │   └────────────────────────────┘  │
                                    │AWS Account: teleport-prod         │
                                    └───────────────────────────────────┘
```

#### **Web Application Firewall**
Amazon [WAF](https://aws.amazon.com/waf/) is a web application firewall that helps protect your web applications. Amazon WAF can be leveraged to implement [rate-based](https://docs.aws.amazon.com/waf/latest/developerguide/waf-rule-statement-type-rate-based.html) rules to prevent the Docker Registry process from being overloaded. Additional rules can be added as seen fit improve the stability and security of the Docker Registry. 

#### **CloudFront**
[CloudFront](https://aws.amazon.com/cloudfront/) will be leveraged to ensure fast, secure, downloads of Teleport OCI images across the globe.

#### Elastic Load Balancer
An internal [Amazon ELB](https://aws.amazon.com/elasticloadbalancing/) will be used to connect CloudFront CDN with whatever we choose to run the registry with. 

#### ECS / EKS / EC2
TODO: Importance and use of either ECS / EKS / EC2

#### S3
TODO: Add comments on S3 as a backend to docker registry. 

Note<sup>*</sup>: A minimal, read-only, _oci-compatible_, registry could be mimicked through CloudFront functions. See alternatives(TODO: Add link to this alternative)

### Implementation
TODO: Include in-depth step by step guide on how the above solution will be created and migrated

* Multi step process. AWS ECR Infrastructure in the `cloud-terraform` repository
* Push to Quay and ECR
* Replicate existing images from quay.io to ECR
* update documentation to new location 
* retire quay repository

### Security Improvements
Image tag immutability
TODO: Add more here or find a way to consolidate in another section. 

### **Alternatives**

#### **Harbor**
Harbor is an open source registry that secures artifacts with policies and role-based access control.<sup>[[2]]</sup> Harbor is a collection of components, the most notable being [Docker Distribution](https://github.com/distribution/distribution), which is an implementation of the [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec). 

Harbor can be installed via [Docker Compose](https://docs.docker.com/compose/) on an EC2 instance or via Helm directly onto a kubernetes cluster for HA. 

Harbor supports authentication via an OIDC single sign-on provider, such as Okta.<sup>[[3]]</sup> 

#### Custom Registry w/ CloudFront Functions
TODO: Thoughts on implementing read-only part of distribution spec to remove middle layer that runs docker registry.
This solution essentially becomes CloudFront + S3 similar to our other artifact stuff.
Additional complexities with pushing the images to S3 as custom script would be needed to make sure the layers are uploaded correctly. 

This solution potentially has the least operational overhead but requires understanding the docker-registry protocol at a decent level. 

#### **Third Party Registry**
TODO: Dockerhub, AWS ECR Public, potential reference to mirroring RFD that would handle this

## References
TODO: Fix or remove broken references list (trying to be fancy)

\[1\] - https://access.redhat.com/articles/5925591
\[2\] - https://goharbor.io/
\[3\] - https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/

[1]: https://access.redhat.com/articles/5925591
[2]: https://goharbor.io/
[3]: https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/

## Brain Dump

* Is discoverability of the images and tags in scope for this RFD? Should users just assume that for each valid release there is a valid tag? Must ensure that release pipeline succeeds or a core release member should be alerted to fix any issues. 
* Make sure to give reference to precedence of existing RFDS such as:
    * [Terraform Registry](https://github.com/gravitational/teleport-plugins/blob/master/rfd/0002-custom-terraform-registry.md)
    * [Release Asset Management](https://github.com/gravitational/cloud/blob/master/rfd/0004-Release-Asset-Management.md)
    * [Package Distribution](https://github.com/gravitational/teleport/pull/10746)
    * [Artifact Storage Standards](https://github.com/gravitational/cloud/blob/master/rfd/0017-artifact-storage-standards.md)

### Out of scope 
* Rate limiting
* Cosign image signing

### In scope
* Secured policy and auditing


What
Why
Details
    Overview
        Address expected questions
            * WHy not dockerhub
        Terminology

    Scope
    Infrastructure
        Individual Components Summaries and Justifications
    Implementation
    Alternatives
    