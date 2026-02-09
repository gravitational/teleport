# What does this do?
This tool takes a built Terraform provider tarball and packages it in the format expected by a Terraform repo. The tarball is expected to only contain the built provider binary itself. This tool converts it to a zip file, creates ".sum" and ".sum.sigs" files, then updates a local copy of an existing registry with the built file.

Additionally, this tool handles Terraform module tarballs.
Module tarballs are packaged in the format expected by Terraform module registry protocol.
Each tarball artifact is renamed into a subdirectory and the module's registry versions index file is updated for module version discovery.

It is up to external processes to create a local copy of the registry, and sync it to S3 if required.

# Terraform Provider Checksum - Testing Locally
As per https://developer.hashicorp.com/terraform/internals/provider-registry-protocol#find-a-provider-package, a shashums_url variable is expected which contains the checksums for the same version for multiple architectures.
All platform metadata will then point to this shared manifest.

To test locally we need to
* set up folders to: source artifacts from, to.
* generate mock artifacts
* prepare a local GPG key
* run tool
* verify contents of sums file, verify signatures.

### Generate folders:
Note: the following commands assume they are being executed from tooling/plugings/cmd/promote-terraform directory

#### Generate source and target directories
```shell
mkdir -p ./local_registry
mkdir -p ./test_artifacts
```

#### Prepare mock artifacts
```shell
# Create unique content for each platform
echo "darwin content" > teleport && tar -czf ./test_artifacts/terraform-provider-teleport-v18.5.1-darwin-arm64-bin.tar.gz teleport
echo "linux content" > teleport && tar -czf ./test_artifacts/terraform-provider-teleport-v18.5.1-linux-amd64-bin.tar.gz teleport
echo "windows content" > teleport && tar -czf ./test_artifacts/terraform-provider-teleport-v18.5.1-windows-amd64-bin.tar.gz teleport

# Clean up the dummy binary
rm teleport
```

#### Prepare local GPG key
```shell
# Generate a key (follow prompts, no passphrase is easiest for local testing)
gpg --full-generate-key

# Export it to a file - this will be used as an argument to run tool
gpg --export-secret-keys --armor "test@example.com" > test_key.asc
```

#### Run tool
```shell
export SIGNING_KEY="$(cat test_key.asc)"

go run . \
  --tag v18.5.1 \
  --artifact-directory-path ./test_artifacts \
  --registry-directory-path ./local_registry \
  --name teleport \
  --namespace gravitational
```

#### Verify contents of the shasums file
```shell
cat local_registry/store/terraform-provider-teleport_18.5.1_SHA256SUMS
```

#### Verify registry - each generated file should point to a master sums file, not a platform specific one
```shell
# This cats every file inside the 'download' subdirectories in registry/gravitational/teleport/<version>/download
find local_registry/registry -type f -not -path "*/store/*" -exec cat {} +
```

#### Verify Signature
````shell
gpg --verify \
  local_registry/store/terraform-provider-teleport_18.5.1_SHA256SUMS.sig \
  local_registry/store/terraform-provider-teleport_18.5.1_SHA256SUMS
````

#### Cleanup
Make sure to remove generated key `test_key.asc` and generated folders: `./local_registry` and `./test_artifacts`