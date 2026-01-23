# What does this do?
This tool takes a built Terraform provider tarball and packages it in the format expected by a Terraform repo. The tarball is expected to only contain the built provider binary itself. This tool converts it to a zip file, creates ".sum" and ".sum.sigs" files, then updates a local copy of an existing registry with the built file.

Additionally, this tool handles Terraform module tarballs.
Module tarballs are packaged in the format expected by Terraform module registry protocol.
Each tarball artifact is renamed into a subdirectory and the module's registry versions index file is updated for module version discovery.

It is up to external processes to create a local copy of the registry, and sync it to S3 if required.
