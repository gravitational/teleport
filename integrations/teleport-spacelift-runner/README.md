# teleport-spacelift-runner

**⚠️ This image is deprecated as of Teleport V18.0.0 and will be removed in V19.0.0 ⚠️**

See https://goteleport.com/docs/admin-guides/infrastructure-as-code/terraform-provider/spacelift/
for instructions on migrating to the new supported way of using the `spacelift`
join method with the Teleport Terraform provider.

---

This Docker image is published for use as a custom image for Spacelift runs.

It is based off the `public.ecr.aws/spacelift/runner-terraform` image and
includes additionally `tbot` and any other dependencies required to run `tbot`.
This allows Machine ID to be used to generate credentials on-the-fly for
managing Teleport Clusters with Terraform.
