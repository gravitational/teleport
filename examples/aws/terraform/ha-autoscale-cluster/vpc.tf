// VPC for Teleport deployment
resource "aws_vpc" "teleport" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Elastic IP for NAT gateways
resource "aws_eip" "nat" {
  for_each = var.az_list

  vpc   = true
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Internet gateway for NAT gateway
resource "aws_internet_gateway" "teleport" {
  vpc_id = aws_vpc.teleport.id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Creates nat gateway per availability zone
resource "aws_nat_gateway" "teleport" {
  for_each      = var.az_list

  allocation_id = aws_eip.nat[each.key].id
  subnet_id     = aws_subnet.public[each.key].id

  depends_on = [
    aws_eip.nat,
    aws_subnet.public,
    aws_internet_gateway.teleport,
  ]
  tags = {
    TeleportCluster = var.cluster_name
  }
}

locals {
  vpc_id              = aws_vpc.teleport.id
  internet_gateway_id = aws_internet_gateway.teleport.id

  # Break up the VPC CIDR into chunks according to different instance type
  # This helps to avoid subnet CIDR conflicts if/when AZs change
  auth_cidr = cidrsubnet(var.vpc_cidr, 4, var.az_subnet_type.auth)
  bastion_cidr = cidrsubnet(var.vpc_cidr, 4, var.az_subnet_type.bastion)
  node_cidr = cidrsubnet(var.vpc_cidr, 4, var.az_subnet_type.node)
  monitor_cidr = cidrsubnet(var.vpc_cidr, 4, var.az_subnet_type.monitor)
  proxy_cidr = cidrsubnet(var.vpc_cidr, 4, var.az_subnet_type.proxy)
}

