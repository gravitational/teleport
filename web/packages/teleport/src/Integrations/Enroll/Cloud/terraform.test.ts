/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { hcl } from './terraform';

describe('hcl', () => {
  test('renders escaped strings', () => {
    const ami = 'ami-12345';
    const instanceType = 't2.micro';

    const result = hcl`
resource "aws_instance" "example" {
  ami           = ${ami}
  instance_type = ${instanceType}
}`;

    expect(result).toBe(`
resource "aws_instance" "example" {
  ami           = "ami-12345"
  instance_type = "t2.micro"
}`);
  });

  test('renders numbers', () => {
    const count = 3;
    const maxPrice = 0.0031;

    const result = hcl`
resource "aws_instance" "example" {
  count = ${count}

  instance_market_options {
    market_type = "spot"
    spot_options {
      max_price = ${maxPrice}
    }
  }
}`;

    expect(result).toBe(`
resource "aws_instance" "example" {
  count = 3

  instance_market_options {
    market_type = "spot"
    spot_options {
      max_price = ${maxPrice}
    }
  }
}`);
  });

  test('renders booleans', () => {
    const enabled = true;
    const disabled = false;

    const result = hcl`
module "example" {
  is_enabled  = ${enabled}
  is_disabled = ${disabled}
}`;

    expect(result).toBe(`
module "example" {
  is_enabled  = ${enabled}
  is_disabled = ${disabled}
}`);
  });

  test('renders arrays with proper formatting', () => {
    const regions = ['us-east-1', 'us-west-2'];

    const result = hcl`
resource "aws_instance" "example" {
  availability_zones = ${regions}
}`;

    expect(result).toBe(`
resource "aws_instance" "example" {
  availability_zones = ["us-east-1", "us-west-2"]
}`);
  });

  test('breaks arrays exceeding 40 chars into multiple lines', () => {
    const moreRegions = [
      'us-east-1',
      'us-west-2',
      'eu-west-1',
      'ap-southeast-1',
      'ca-central-1',
    ];

    const result = hcl`
module "region_example" {
  example_regions = ${moreRegions}
}`;

    expect(result).toBe(`
module "region_example" {
  example_regions = [
    "us-east-1",
    "us-west-2",
    "eu-west-1",
    "ap-southeast-1",
    "ca-central-1"
  ]
}`);
  });

  test('renders objects with proper indentation and alignment', () => {
    const tags = {
      Environment: 'production',
      Team: 'infrastructure',
      Project: 'teleport',
    };

    const result = hcl`
resource "aws_instance" "example" {
  tags = ${tags}
}`;

    expect(result).toBe(`
resource "aws_instance" "example" {
  tags = {
    Environment = "production"
    Team        = "infrastructure"
    Project     = "teleport"
  }
}`);
  });

  test('quotes object keys when necessary', () => {
    const tags = {
      'dash-identifier-ok': 'value1',
      'with spaces': 'value2',
      '123startswithnumber': 'value3',
      valid_key: 'value4',
      normal: 'value5',
    };

    const result = hcl`
resource "aws_instance" "example" {
  tags = ${tags}
}`;

    expect(result).toBe(`
resource "aws_instance" "example" {
  tags = {
    dash-identifier-ok    = "value1"
    "with spaces"         = "value2"
    "123startswithnumber" = "value3"
    valid_key             = "value4"
    normal                = "value5"
  }
}`);
  });

  test('removes preceding lines with only whitespace or comments when null value used', () => {
    const regions = null;
    const tags = { Environment: 'test' };

    const result = hcl`
module "test_module" {
    
  # remove me  
  regions = ${regions}

  # this stays
  tags = ${tags}
}`;

    expect(result).toBe(`
module "test_module" {

  # this stays
  tags = {
    Environment = "test"
  }
}`);
  });

  test('preserves indentation with nested structures', () => {
    const config = {
      vpc: {
        cidr_block: '10.0.0.0/16',
        enable_dns: true,
      },
      subnets: ['10.0.1.0/24', '10.0.2.0/24'],
    };

    const result = hcl`
resource "aws_vpc" "main" {
  network_config = ${config}
}`;

    expect(result).toBe(`
resource "aws_vpc" "main" {
  network_config = {
    vpc     = {
      cidr_block = "10.0.0.0/16"
      enable_dns = true
    }
    subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  }
}`);
  });

  test('renders empty arrays and objects', () => {
    const emptyArray = [];
    const emptyObject = {};

    const result = hcl`
resource "aws_instance" "example" {
  security_groups = ${emptyArray}
  tags            = ${emptyObject}
}`;

    expect(result).toBe(`
resource "aws_instance" "example" {
  security_groups = []
  tags            = {}
}`);
  });

  test('handles multiline string values', () => {
    const value = `hello
  world`;

    const result = hcl`
resource "multiline_string" "example" {
  example = ${value}
}`;

    expect(result).toBe(`
resource "multiline_string" "example" {
  example = "hello\\n  world"
}`);
  });

  test('handles example terraform module configuration', () => {
    const integrationName = 'my-integration';
    const clusterName = 'teleport.example.com';
    const matchAwsTypes = ['ec2'];
    const regions = null;
    const tags = {
      Environment: 'production',
      Owner: 'platform-team',
    };

    const result = hcl`# Terraform Module
module "test_module" {
  teleport_integration_name     = ${integrationName}
  teleport_cluster_name         = ${clusterName}

  match_aws_types  = ${matchAwsTypes}

  # AWS regions
  match_aws_regions = ${regions}
  match_aws_tags = ${tags}
}`;

    expect(result).toBe(`# Terraform Module
module "test_module" {
  teleport_integration_name     = "my-integration"
  teleport_cluster_name         = "teleport.example.com"

  match_aws_types  = ["ec2"]
  match_aws_tags = {
    Environment = "production"
    Owner       = "platform-team"
  }
}`);
  });
});
