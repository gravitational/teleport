# What does this do?
This tool takes a built Terraform provider tarball and packages it in the format expected by a Terraform repo. The tarball is expected to only contain the built provider binary itself. This tool converts it to a zip file, creates ".sum" and ".sum.sigs" files, then updates a local copy of an existing registry with the built file.

It is up to external processes to create a local copy of the registry, and sync it to S3 if required.
