# teleport-spacelift-runner

This Docker image is published for use as a custom image for Spacelift runs.

It is based off the `public.ecr.aws/spacelift/runner-terraform` image and
includes additionally `tbot` and any other dependencies required to run `tbot`.
This allows Machine ID to be used to generate credentials on-the-fly for
managing Teleport Clusters with Terraform.

See https://goteleport.com/docs/machine-id/deployment/spacelift for additional
information.