/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { ResourceLabel } from 'teleport/services/agents';

import { getRoleYaml } from './useGitHubSshFlow';

// indentOf returns the number of leading spaces before `key:` on its own
// line, so tests can assert on the YAML's nesting without pulling in a full
// YAML parser dependency.
function indentOf(yaml: string, key: string): number {
  const match = yaml.match(new RegExp(`^( *)${key}:`, 'm'));
  if (!match) {
    throw new Error(`key "${key}:" not found in generated role YAML`);
  }
  return match[1].length;
}

describe('getRoleYaml', () => {
  const labels: ResourceLabel[] = [{ name: 'env', value: 'prod' }];

  it('places `options` as a sibling of `allow`, not nested inside it', () => {
    const yaml = getRoleYaml('test-bot', labels, 'ubuntu');

    // `options` is a field of the role spec itself, alongside `allow`/`deny`,
    // not a field of `allow`. Regression test for a "role has unknown or
    // misspelled fields: json: unknown field \"options\"" error on apply.
    expect(indentOf(yaml, 'options')).toBe(indentOf(yaml, 'allow'));
    expect(indentOf(yaml, 'max_session_ttl')).toBeGreaterThan(
      indentOf(yaml, 'options')
    );
  });

  it('includes the provided bot name, login, and node labels', () => {
    const yaml = getRoleYaml('test-bot', labels, 'ubuntu');

    expect(yaml).toContain('name: test-bot');
    expect(yaml).toContain('logins: [ubuntu]');
    expect(yaml).toContain("'env': 'prod'");
  });
});
