---
authors: Logan Davis (logan.davis@goteleport.com)
state: draft
---

# RFD 58 - Public OCI Images

## **What**

Teleport images are currently hosted on [Quay.io](https://quay.io/organization/gravitational). This RFD proposes migrating public images from Quay to a self-hosted [Harbor](https://goharbor.io/) registry. 

## **Why**

As of August 1st, 2021 Quay.io no longer supports any other authentication provider other than Red Hat Single-Sign On.<sup>[[1]]</sup> Users in the Gravitational organization on Quay must be manually removed when they leave Teleport which presents a potential security risk. Migrating to a solution with support for our existing SSO infrastructure helps remediate this issue.

## **Details**
In this RFD, we propose migrating from Quay to [Harbor](https://goharbor.io/).

Moving our public image infrastructure from Quay to Harbor gives us improved security controls with support for:
* [SSO through Okta](https://goharbor.io/docs/2.5.0/administration/configure-authentication/oidc-auth/)
* [Tag immutability](https://goharbor.io/docs/2.5.0/working-with-projects/working-with-images/create-tag-immutability-rules/)
* [Audit Logs](https://goharbor.io/docs/2.5.0/working-with-projects/project-configuration/access-project-logs/)
* Support for multiple [vulnerability scanners](https://goharbor.io/docs/2.5.0/administration/vulnerability-scanning/)
* [Robot Accounts](https://goharbor.io/docs/2.5.0/administration/robot-accounts/) with various permission combinations
* And much more... 

**Why not use Dockerhub or other equivalents?**

There is existing precedence within Teleport for maintaining close control of the distribution of our software. See:
* Cloud [RFD 0004](https://github.com/gravitational/cloud/blob/master/rfd/0004-Release-Asset-Management.md)
* Terraform Registry [RFD 0002](https://github.com/gravitational/teleport-plugins/blob/master/rfd/0002-custom-terraform-registry.md)

Running our own container registry maximizes the control we have over the distribution of our software.

**What about name squatting on other image registries?**

A not yet finished RFD on third-party artifact mirroring will address concerns regarding name squatting on other container registry platforms. See [Artifact Mirroring](https://github.com/gravitational/teleport/commit/2262efbb25463ccc135553d43293f6d8aee22ba2).

Any additional concerns are beyond the scope of this RFD. 

### **Scope**

This RFD will focus on the why and how of using Harbor as our public image container registry. This RFD will aim for complete feature parity with the existing Quay.io solution in addition to improved security with Okta SSO and tag immutability. Addiitonal security features, although possible, will be outside the scope of this RFD. 

This RFD will also include a detailed deprecation plan for the existing Quay.io registry. 

### **Infrastructure**
Hosting our own _oci-compatible_ registry is similar to hosting our own [terraform registry](https://github.com/gravitational/teleport-plugins/blob/master/rfd/0002-custom-terraform-registry.md). However, the [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec) has additional complexities that can't be solved by S3 and CloudFront<sup>*</sup> alone. These complexities warrant the use of [Harbor](https://goharbor.io/).

Note<sup>*</sup>: A minimal, read-only, _oci-compatible_, registry could be mimicked through CloudFront functions. See alternatives.

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
                                    │   │  │         EKS           │ │  │
                                    │   │  │ ┌───────────────────┐ │ │  │
                                    │   │  │ │      Harbor       │ │ │  │
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
Amazon [WAF](https://aws.amazon.com/waf/) is a web application firewall that helps protect your web applications. Amazon WAF can be leveraged to implement [rate-based](https://docs.aws.amazon.com/waf/latest/developerguide/waf-rule-statement-type-rate-based.html) rules to prevent Harbor from being overloaded. Additional rules can be added as seen fit improve the stability and security of the registry. 

#### **CloudFront**
[CloudFront](https://aws.amazon.com/cloudfront/) will be leveraged to ensure fast, secure, downloads of Teleport OCI images across the globe.

#### **Elastic Load Balancer**
An internal [Amazon ELB](https://aws.amazon.com/elasticloadbalancing/) will be used to connect CloudFront CDN with the harbor process running within EKS. 

#### **EKS**
An Amazon EKS cluster will be deployed via IaC managed in the [cloud-terraform](https://github.com/gravitational/cloud-terraform/) repository. In order to reduce operational complexity, this cluster will exist within the `teleport-prod` AWS account alongside other resources. 

An alternative to deploying this cluster within the `teleport-prod` AWS account would be to deploy this to an alternative AWS account and use cross-account IAM policies to grant access to the S3 bucket within `teleport-prod`. Deploying to a separate account would provide security isolation by account separation instead of through least-privileged IAM policies. 

#### **Harbor**
Harbor supports deployment through either `docker compose` or via `helm charts`. Harbor will be installed into a managed EKS cluster using helm as part of the implementation steps. Leveraging the Helm [terraform provider](https://registry.terraform.io/providers/hashicorp/helm/latest/docs), all changes to the Harbor installation will be managed with IaC. All changes will be peer-reviewed within the [cloud-terraform](https://github.com/gravitational/cloud-terraform/) repository.

#### **S3**
Harbor supports using S3 as a storage backend for images. Amazon S3 will be leveraged for this purpose. An S3 bucket will be created in the `teleport-prod` AWS account and will strictly follow standards as set in [Artifact Storage Standards](https://github.com/gravitational/cloud/blob/master/rfd/0017-artifact-storage-standards.md). The Harbor process should be the only service that needs access to modify this bucket. All container operations will occur through Harbor.

### **Security Considerations**

#### **Hosting**
All resources laid out in this RFD will be hosted in the `teleport-prod` AWS account as defined by the [Artifact Storage Standards](https://github.com/gravitational/cloud/blob/master/rfd/0017-artifact-storage-standards.md) RFD. All resources will be configured and deployed using IaC with Terraform in the [cloud-terraform](https://github.com/gravitational/cloud-terraform/) repository.

#### **Patching Harbor**
Updates to the Harbor environment will be performed by the Core tooling and/or Release Engineers. Cloud tooling and/or Release Engineers can/will assist in this procedure as needed. 

#### **Application Access using Teleport**
Secure access to the Harbor UI can be controlled through [Teleport Application Access](https://goteleport.com/docs/application-access/introduction/). The `platform.teleport.sh` cluster would be used to control this access. 

### **Alternatives**

#### **Custom Registry w/ CloudFront Functions**
A minimal, _oci-compatible_ registry could be implemented through just CloudFront functions. This registry would only support reading. This would reduce the operational complexity of the current strategy to AWS specific components. Additional components would be needed to be developed in order to push the image to the S3 bucket but could be implemented as just another step in the existing CI/CD pipeline.

A negative to this solution is the lack of features that come standard with other registry solutions. This includes, but is not limited to, vulnerability scanning and tag immutability. 

#### **Third-Party Registries**
While both third-party registries presented below would be less overhead to manage. They both involve losing control over the distribution of our software, which is a key point that this RFD and other RFDs encourage. 

However, in order to address the issue of name squatting, and to provide customers a potentially improved experience, an RFD on [Artifact Mirroring](https://github.com/gravitational/teleport/commit/2262efbb25463ccc135553d43293f6d8aee22ba2) will be created. 

**Dockerhub**

Docker Hub supports bringing your own [SSO provider](https://docs.docker.com/single-sign-on/) and [vulnerability scanning](https://docs.docker.com/docker-hub/vulnerability-scanning/). This choice would produce the strongest user experience as images could be named `gravitational/teleport` or even just `teleport` if Docker allowed. 

**Amazon ECR Public**

Amazon ECR Public supports [image scanning](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-scanning.html) and would leverage our existing Okta SSO infrastructure. Additionally, repositories would be managed using Terraform IaC. However, Amazon ECR Public doesn't have support for custom domains. This means images would be pulled using `public.ecr.aws/gravitational/teleport`.

### **Implementation**
The following steps will be followed 

* Multi step process. AWS ECR Infrastructure in the `cloud-terraform` repository
* Push to Quay and Harbor
* Replicate existing images from quay.io to ECR
* update documentation to new location 
* retire quay repository

## **References**
\[1\] - https://access.redhat.com/articles/5925591
\[2\] - https://goharbor.io/
\[3\] - https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/

[1]: https://access.redhat.com/articles/5925591
[2]: https://goharbor.io/
[3]: https://goharbor.io/docs/2.4.0/administration/configure-authentication/oidc-auth/

## Appendix A
The following table represents a best guess guide to migration of existing images from Quay to Harbor. They have been marked as such given their perceived relevance based on Quay activity and number of references in the Gravitational organization. 
Key:
* **Y**: Will Migrate
* **N**: Won't Migrate
* **U**: Unsure

| Repository | Migration | Repository | Migration | Repository | Migration | Repository | Migration |
| ---- | ---- | ---- | ---- | ---- | ---- | ---- | ---- |
| teleport | Y | netbox | N | sync-kubeconfig | N | sync-kubesecrets | N |
| teleport-ent | Y | debian-venti | Y | stress | N | docker-gc | N |
| teleport-buildbox | Y | debian-grande | Y | robotest | N | robotest-e2e | N |
| next | Y | teleport-buildbox-centos6 | N | robotest-suite | Y | mattermost-worker | N |
| rig | U | fpm-centos | Y | keygen | N | busyloop | N |
| debian-tall | Y | fpm-debian | Y | mkdocs-base | Y | teleport-ent-dev | U |
| teleport-buildbox-arm | Y | teleport-buildbox-fips | Y | slackbot | Y | cve-2018-1002105 | N |
| teleport-lab | Y |  prometheus-operator | Y | wormhole-dev | U | kaniko-init-container | N |
| buildbox-base | Y | kube-router | Y | watcher | N | nginx-controller | N |
| satellite | Y | teleport-plugin-slack | Y | stolon-pgbouncer | N | pithos | N |
| planet | Y | ubuntu-venti | Y | stolon | N | stolon-etcd | N |
| wormhole | Y | nethealth-dev | N | teleport-buildbox-centos6-fips | N | stolonctl | N |
| teleport-plugin-email | Y | provisioner | N | pithosctl | N | s3-mounter | N |
| ubuntu-tall | Y | ubuntu-grande | Y | alpine | N | force | N |
| teleport-plugin-jira | Y | teleport-plugin-mattermost | Y | pithos-proxy | N | gravity-scan | N |
| teleport-plugin-pagerduty | Y | teleport-plugin-event-handler | Y | monitoring-grafana | N | docker-alpine-build | N |
| nginx | N | tube | N | alpine-glibc | N | monitoring-influxdb | N |
| teleport-dev | N | teleport-buildbox-arm-fips | Y | drone-fork-approval-extension | N | aws-ecr-helper | Y |


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
* Critical Image alerting? (Scanning with this custom solution requires custom scanning)
* Discoverability

### In scope
* RFD 17 adherence
* Future of Quay and potential degradation plan (does mirroring rfd play into this)

Appendix of all docker repos in quay and triage
    * what are customer facing and need mirrroing
    * what to delete / deprecate
    * port but are internal only 

Can call out to appendix in the scope section.

Matrix of docker-registry features needed

Isolation mechanisms for containing harbor. Example IAM policies or separate account with cross account roles. 

What
Why
Details
    Overview
        Address expected questions
            * WHy not dockerhub


    Scope
    Infrastructure
        Individual Components Summaries and Justifications
    Implementation
    Alternatives
    