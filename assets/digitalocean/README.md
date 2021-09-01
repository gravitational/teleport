## Teleport Digital Ocean marketplace images.

This repository contains scripts and templates to build a 1-click Teleport application for Digital Ocean marketplace. The snapshot building process is based on [Packer](https://www.packer.io/intro/index.html) so make sure it is installed in your system.

## Files
- `template.json` is the main configuration template that is used to build snapshots using Packer.
- `scripts/` has the teleport installation script.
- `files/` has scripts required to configure and update droplet VM. It also has a script to configure teleport on first time login via SSH.
- `common/` has files that are required to build VM image.
- `assets/` is only there to hold reference and dependencies that may be used or required in the future. 


## Usage

### Step 1: Prepare Digital Ocean access token.
Create personal access token from Digital Ocean control panel (in API menu).

export the token: `export DIGITALOCEAN_API_TOKEN={token}`

### Step 2: Building snapshot

1) Validate the template: `$ packer validate template.json`
2) Build the snapshot: `$ packer build template.json`

Packer will create a snapshot image based on `template.json` file.

### Step 3: Test the snapshot
Go to Digital Ocean control panel and create a droplet based on the newly created snapshot.


## Notes
If you are creating a new snapshot, make sure the `common/scripts/999-img_check.sh` and `common/scripts/900-cleanup.sh` files are up to date. Canonical version for these files are in [Marketplace Partner](https://github.com/digitalocean/marketplace-partners/tree/master/scripts) repository.


For more reference, please check [droplet-1-clicks](https://github.com/digitalocean/droplet-1-clicks) repositories that hold packer files used to build 1-Click applications published by Digital Ocean. This project heavily follows conventions and configurations found in that repository.