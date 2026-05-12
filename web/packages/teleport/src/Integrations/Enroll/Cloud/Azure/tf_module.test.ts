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
  AzureDiscoverTerraformModuleConfig,
  buildTerraformConfig,
} from './tf_module';

describe('buildTerraformConfig', () => {
  const baseConfig: AzureDiscoverTerraformModuleConfig = {
    integrationName: 'my-integration',
    version: '19.0.0',
    vmConfig: {
      type: 'vm',
      enabled: true,
      regions: ['eastus'],
      subscriptions: [],
      resourceGroups: [],
      tags: [],
    },
    managedIdentity: {
      resourceGroup: 'my-resource-group',
      region: 'eastus',
    },
  };

  test('uses production registry for release versions', () => {
    const result = buildTerraformConfig(baseConfig);

    expect(result).toContain(
      'source  = "terraform.releases.teleport.dev/teleport/discovery/azure"'
    );
  });

  test('uses staging registry for prerelease versions', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      version: '19.0.0-rc.1',
    });

    expect(result).toContain(
      'source  = "terraform-staging.releases.development.teleport.dev/teleport/discovery/azure"'
    );
  });

  test('includes vm type in matcher', () => {
    const result = buildTerraformConfig(baseConfig);

    expect(result).toMatch(/types\s+=\s+\["vm"\]/);
  });

  test('omits azure_matchers when vm is disabled', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: { ...baseConfig.vmConfig, enabled: false },
    });

    expect(result).not.toContain('azure_matchers');
  });

  test('includes specific regions sorted', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: { ...baseConfig.vmConfig, regions: ['westus', 'eastus'] },
    });

    expect(result).toMatch(/regions\s+=\s+\["eastus",\s+"westus"\]/);
  });

  test('omits regions for wildcard', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: { ...baseConfig.vmConfig, regions: ['*'] },
    });

    expect(result).not.toContain('regions');
  });

  test('includes subscriptions sorted', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: { ...baseConfig.vmConfig, subscriptions: ['sub-b', 'sub-a'] },
    });

    expect(result).toContain('subscriptions = ["sub-a", "sub-b"]');
  });

  test('includes empty subscriptions array when none provided', () => {
    const result = buildTerraformConfig(baseConfig);

    expect(result).toContain('subscriptions = []');
  });

  test('includes resource_groups sorted', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: { ...baseConfig.vmConfig, resourceGroups: ['rg-2', 'rg-1'] },
    });

    expect(result).toContain('resource_groups = ["rg-1", "rg-2"]');
  });

  test('omits resource_groups when empty', () => {
    const result = buildTerraformConfig(baseConfig);

    expect(result).not.toContain('resource_groups');
  });

  test('includes tags', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: {
        ...baseConfig.vmConfig,
        tags: [{ name: 'env', value: 'prod' }],
      },
    });

    expect(result).toContain('env = ["prod"]');
  });

  test('omits tags when empty', () => {
    const result = buildTerraformConfig(baseConfig);

    expect(result).not.toContain('tags');
  });

  test('omits tags for wildcard tag name', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: {
        ...baseConfig.vmConfig,
        tags: [{ name: '*', value: '*' }],
      },
    });

    expect(result).not.toContain('tags');
  });

  test('collects multiple values for the same tag key', () => {
    const result = buildTerraformConfig({
      ...baseConfig,
      vmConfig: {
        ...baseConfig.vmConfig,
        tags: [
          { name: 'env', value: 'prod' },
          { name: 'env', value: 'staging' },
        ],
      },
    });

    expect(result).toContain('env = ["prod", "staging"]');
  });
});
