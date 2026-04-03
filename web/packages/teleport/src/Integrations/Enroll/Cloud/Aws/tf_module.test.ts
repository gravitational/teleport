/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import {
  buildTerraformConfig,
  AwsDiscoverTerraformModuleConfig,
} from './tf_module';
import type { AwsMatcher } from './types';

describe('buildTerraformConfig', () => {
  const ec2WithRegionAndTags: AwsMatcher = {
    type: 'ec2',
    regions: ['us-east-1'],
    tags: [{ name: 'env', value: 'prod' }],
  };

  const baseConfig: AwsDiscoverTerraformModuleConfig = {
    integrationName: 'my-integration',
    matchers: [ec2WithRegionAndTags],
    version: '19.0.0',
  };

  test('uses production registry for release versions', () => {
    const result = buildTerraformConfig(baseConfig);

    expect(result).toContain(
      'source  = "terraform.releases.teleport.dev/teleport/discovery/aws"'
    );
  });

  test('uses staging registry for prerelease versions', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      version: '19.0.0-rc.1',
    });

    expect(result).toContain(
      'source  = "terraform-staging.releases.development.teleport.dev/teleport/discovery/aws"'
    );
  });

  test('EC2 only with specific regions and tags', () => {
    const result = buildTerraformConfig(baseConfig);

    expect(result).toContain('aws_matchers');
    expect(result).toContain('types   = ["ec2"]');
    expect(result).toContain('regions = ["us-east-1"]');
    expect(result).toContain('env = ["prod"]');
    expect(result).not.toContain('eks');
  });

  test('EC2 with no region selection uses wildcard regions', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [{ type: 'ec2', regions: [], tags: [] }],
    });

    expect(result).toContain('types   = ["ec2"]');
    expect(result).toContain('regions = ["*"]');
    expect(result).not.toContain('tags');
  });

  test('EKS with no region selection does not use wildcard regions', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [{ type: 'eks', regions: [], tags: [] }],
    });

    expect(result).toContain('types   = ["eks"]');
    expect(result).not.toContain('regions');
  });

  test('EKS only with specific regions and tags', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [
        {
          type: 'eks',
          regions: ['us-west-2'],
          tags: [{ name: 'team', value: 'platform' }],
        },
      ],
    });

    expect(result).toContain('types   = ["eks"]');
    expect(result).toContain('regions = ["us-west-2"]');
    expect(result).toContain('team = ["platform"]');
    expect(result).not.toContain('"ec2"');
  });

  test('EC2 + EKS with different regions and tags', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [
        {
          type: 'ec2',
          regions: ['us-east-1'],
          tags: [{ name: 'env', value: 'prod' }],
        },
        {
          type: 'eks',
          regions: ['us-west-2'],
          tags: [{ name: 'team', value: 'platform' }],
        },
      ],
    });

    expect(result).toContain('types   = ["ec2"]');
    expect(result).toContain('types   = ["eks"]');
    expect(result).toContain('env = ["prod"]');
    expect(result).toContain('team = ["platform"]');
  });

  test('omits tags when no tags specified', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [{ type: 'ec2', regions: ['us-east-1'], tags: [] }],
    });

    expect(result).toContain('types   = ["ec2"]');
    expect(result).not.toContain('tags');
  });

  test('omits wildcard tags', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [
        {
          type: 'ec2',
          regions: ['us-east-1'],
          tags: [{ name: '*', value: '*' }],
        },
      ],
    });

    expect(result).not.toContain('tags');
  });

  test('tags with multiple values for same key', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [
        {
          type: 'ec2',
          regions: ['us-east-1'],
          tags: [
            { name: 'env', value: 'prod' },
            { name: 'env', value: 'staging' },
          ],
        },
      ],
    });

    expect(result).toContain('env = ["prod", "staging"]');
  });

  test('no matchers produces no aws_matchers', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [],
    });

    expect(result).not.toContain('aws_matchers');
  });

  test('regions are sorted alphabetically', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      matchers: [
        {
          type: 'ec2',
          regions: ['us-west-2', 'eu-west-1', 'us-east-1'],
          tags: [],
        },
      ],
    });

    expect(result).toContain(
      'regions = ["eu-west-1", "us-east-1", "us-west-2"]'
    );
  });
});
