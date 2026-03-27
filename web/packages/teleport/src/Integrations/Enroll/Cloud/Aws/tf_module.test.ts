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

import type { Regions } from 'teleport/services/integrations';

import {
  buildTerraformConfig,
  AwsDiscoverTerraformModuleConfig,
} from './tf_module';
import type { ServiceConfigs } from './types';

describe('buildTerraformConfig', () => {
  const disabled = { enabled: false, regions: ['*'] as ['*'], tags: [] };

  const baseConfigs: ServiceConfigs = {
    ec2: {
      enabled: true,
      regions: ['us-east-1'] as Regions[],
      tags: [{ name: 'env', value: 'prod' }],
    },
    eks: disabled,
  };

  const baseConfig: AwsDiscoverTerraformModuleConfig = {
    integrationName: 'my-integration',
    configs: baseConfigs,
    version: '19.0.0',
  };

  const withConfigs = (
    overrides: Partial<ServiceConfigs>
  ): AwsDiscoverTerraformModuleConfig => ({
    ...baseConfig,
    configs: { ...baseConfigs, ...overrides },
  });

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

  test('EC2 with wildcard regions omits regions field', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: { enabled: true, regions: ['*'], tags: [] },
      })
    );

    expect(result).toContain('types = ["ec2"]');
    expect(result).not.toContain('regions');
    expect(result).not.toContain('tags');
  });

  test('EKS only with specific regions and tags', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: disabled,
        eks: {
          enabled: true,
          regions: ['us-west-2'] as Regions[],
          tags: [{ name: 'team', value: 'platform' }],
        },
      })
    );

    expect(result).toContain('types   = ["eks"]');
    expect(result).toContain('regions = ["us-west-2"]');
    expect(result).toContain('team = ["platform"]');
    expect(result).not.toContain('"ec2"');
  });

  test('EC2 + EKS with different regions and tags', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: {
          enabled: true,
          regions: ['us-east-1'] as Regions[],
          tags: [{ name: 'env', value: 'prod' }],
        },
        eks: {
          enabled: true,
          regions: ['us-west-2'] as Regions[],
          tags: [{ name: 'team', value: 'platform' }],
        },
      })
    );

    expect(result).toContain('types   = ["ec2"]');
    expect(result).toContain('types   = ["eks"]');
    expect(result).toContain('env = ["prod"]');
    expect(result).toContain('team = ["platform"]');
  });

  test('omits tags when no tags specified', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: {
          enabled: true,
          regions: ['us-east-1'] as Regions[],
          tags: [],
        },
      })
    );

    expect(result).toContain('types   = ["ec2"]');
    expect(result).not.toContain('tags');
  });

  test('omits wildcard tags', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: {
          enabled: true,
          regions: ['us-east-1'] as Regions[],
          tags: [{ name: '*', value: '*' }],
        },
      })
    );

    expect(result).not.toContain('tags');
  });

  test('tags with multiple values for same key', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: {
          enabled: true,
          regions: ['us-east-1'] as Regions[],
          tags: [
            { name: 'env', value: 'prod' },
            { name: 'env', value: 'staging' },
          ],
        },
      })
    );

    expect(result).toContain('env = ["prod", "staging"]');
  });

  test('neither service enabled produces no aws_matchers', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: disabled,
        eks: disabled,
      })
    );

    expect(result).not.toContain('aws_matchers');
  });

  test('regions are sorted alphabetically', () => {
    const result = buildTerraformConfig(
      withConfigs({
        ec2: {
          enabled: true,
          regions: ['us-west-2', 'eu-west-1', 'us-east-1'] as Regions[],
          tags: [],
        },
      })
    );

    expect(result).toContain(
      'regions = ["eu-west-1", "us-east-1", "us-west-2"]'
    );
  });
});
