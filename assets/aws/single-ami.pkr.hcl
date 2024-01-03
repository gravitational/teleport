# Teleport
# Copyright (C) 2023  Gravitational, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

packer {
  required_plugins {
    amazon = {
      version = "1.2.5"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

variable "aws_account_id" {
  type        = string
  description = "The ID number of the account that will build and own the AMI"
  validation {
    condition = can(regex("^\\d{12}$", var.aws_account_id))
    error_message = "Invalid AWS account ID. Must be exactly 12 digits."
  }
}

variable "aws_region" {
  type    = string
  default = "us-west-2"
}

variable "aws_vpc_id" {
  type = string
  description = "ID of the AWS VPC used to house the builder instances."
  validation {
    condition = can(regex("^vpc-[[:alnum:]]{17,}", var.aws_vpc_id))
    error_message = "Invalid AWS VPC ID."
  }
}

variable "aws_instance_type" {
  type    = string
  description = "Type of EC2 instance used to provision AMI."
}

variable "teleport_type" {
  type        = string
  description = "oss | ent"
  validation {
    condition     = contains(["oss", "ent"], var.teleport_type)
    error_message = "Unsupported Teleport type."
  }
}

variable "teleport_version" {
  type = string
}

variable "teleport_fips" {
  type    = bool
  default = false
}

variable "teleport_tarball" {
  description = "Path to teleport tarball"
  type = string
}

variable "teleport_uid" {
  type    = string
  default = "1007"
}

variable "ami_build_timestamp" {
  type = string
}

variable "ami_name" {
  type    = string
  default = ""
}

variable "ami_arch" {
  type    = string
  default = ""
}

variable "ami_destination_regions" {
  type    = string
  default = "us-west-2"
}

data "amazon-ami" "teleport-hardened-base" {
  filters = {
    name                = "teleport-hardened-base-image-${var.ami_arch}-al2023-*"
    root-device-type    = "ebs"
    virtualization-type = "hvm"
  }
  most_recent = true
  owners      = [var.aws_account_id]
  region      = var.aws_region
}

locals {
  # apply a default AMI name if no name was specified on the command line.
  unsafe_ami_name = var.ami_name != "" ? var.ami_name : "teleport-debug-ami-${var.teleport_type}-${var.teleport_version}"

  # sanitize the AMI name so that it's safe for use with AWS
  ami_name = regex_replace(local.unsafe_ami_name, "[^a-zA-Z0-9\\- \\(\\).\\'[\\]@]", "-")

  # split the comma-separated region list out into a proper array
  destination_regions = [for s in split(",", var.ami_destination_regions) : trimspace(s)]

  ami_description = "Teleport${var.teleport_fips ? " with FIPS support" : ""} using Hardened Amazon Linux 2023 (${var.ami_arch}) AMI"
  build_type      = "production${var.teleport_fips ? "-fips" : ""}"

  # Used in AWS access policies. Do not change without consulting the teleport-prod
  # terraform.
  resource_purpose_tag_value = "release-ami-builder"

  # We can't execute from /tmp due to CIS hardening guidelines, so we have to
  # specify a place to run provisioning scripts
  remote_folder = "/home/ec2-user"
}

source "amazon-ebs" "teleport-aws-linux" {
  ami_description                           = local.ami_description
  ami_name                                  = local.ami_name
  ami_regions                               = local.destination_regions
  associate_public_ip_address               = true
  temporary_security_group_source_public_ip = true
  force_delete_snapshot                     = true
  instance_type                             = var.aws_instance_type
  region                                    = var.aws_region
  encrypt_boot                              = false
  imds_support                              = "v2.0"
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 2
  }
  run_tags = {
    Name                     = local.ami_name
    purpose                  = local.resource_purpose_tag_value
    "teleport.dev/is_public" = true
  }
  run_volume_tags = {
    Name = local.ami_name
  }
  snapshot_tags = {
    Name = local.ami_name
  }
  source_ami   = data.amazon-ami.teleport-hardened-base.id
  ssh_pty      = true
  ssh_username = "ec2-user"
  subnet_filter {
    filters = {
      "tag:purpose"     = local.resource_purpose_tag_value
      "tag:environment" = "prod"
    }
    most_free = true
  }
  tags = {
    BuildTimestamp      = var.ami_build_timestamp
    BuildType           = "production"
    Name                = local.ami_name
    Architecture        = var.ami_arch
    TeleportVersion     = var.teleport_version
    TeleportEdition     = var.teleport_type
    TeleportFipsEnabled = var.teleport_fips
  }
  vpc_id = var.aws_vpc_id
}

build {
  sources = [
    "source.amazon-ebs.teleport-aws-linux",
  ]

  provisioner "shell" {
    remote_folder = local.remote_folder
    inline = ["mkdir /tmp/files"]
  }

  provisioner "file" {
    source      = "files/"
    destination = "/tmp/files"
  }

  provisioner "file" {
    source = var.teleport_tarball
    destination = "/tmp/teleport.tar.gz"
  }

  provisioner "shell" {
    remote_folder = local.remote_folder
    inline = [
      "sudo cp /tmp/files/system/* /etc/systemd/system/",
      "sudo cp /tmp/files/bin/* /usr/local/bin/"
    ]
  }

  provisioner "shell" {
    remote_folder = local.remote_folder
    environment_vars = [
      "TELEPORT_UID=${var.teleport_uid}",
      "TELEPORT_VERSION=${var.teleport_version}",
      "TELEPORT_TYPE=${var.teleport_type}",
      "TELEPORT_FIPS=${var.teleport_fips ? 1 : 0}"
    ]
    execute_command = "chmod +x {{ .Path }}; echo 'root' | {{ .Vars }} sudo -S -E bash -eux '{{ .Path }}'"
    script          = "files/install-hardened.sh"
  }
}
