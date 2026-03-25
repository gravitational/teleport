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

import { buildTerraformConfig } from './tf_module';

describe('buildTerraformConfig', () => {
  const regions: Regions[] = ['us-east-1'];

  const baseConfig = {
    integrationName: 'my-integration',
    regions,
    ec2Config: {
      enabled: true,
      tags: [{ name: 'env', value: 'prod' }],
    },
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
});
