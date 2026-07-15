/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
  ResourceConstraints,
  ResourceIDString,
} from 'shared/services/accessRequests';

import {
  formatAWSRoleARNForDisplay,
  toggleAWSConsoleConstraint,
  toggleSSHConstraint,
} from './utils';

describe('formatAWSRoleARNForDisplay', () => {
  it('formats a valid ARN as "accountId: rolePathAndName"', () => {
    expect(
      formatAWSRoleARNForDisplay('arn:aws:iam::123456789012:role/Admin')
    ).toBe('123456789012: Admin');
  });

  it('handles path-based role names', () => {
    expect(
      formatAWSRoleARNForDisplay(
        'arn:aws:iam::123456789012:role/path/to/MyRole'
      )
    ).toBe('123456789012: path/to/MyRole');
  });

  it('returns the original string for non-matching ARNs', () => {
    expect(formatAWSRoleARNForDisplay('not-an-arn')).toBe('not-an-arn');
  });
});

describe('toggleAWSConsoleConstraint', () => {
  it('removes the specified ARN and keeps remaining', () => {
    const set = jest.fn();
    toggleAWSConsoleConstraint(
      {
        id: 'my-app',
        kind: 'app' as const,
        clusterName: 'cluster',
        constraints: {
          aws_console: {
            role_arns: ['arn:1', 'arn:2'],
          },
        },
      },
      'arn:1',
      set
    );
    expect(set).toHaveBeenCalledWith('cluster/app/my-app' as ResourceIDString, {
      aws_console: { role_arns: ['arn:2'] },
    });
  });

  it('clears constraints when last ARN is removed', () => {
    const set = jest.fn();
    toggleAWSConsoleConstraint(
      {
        id: 'my-app',
        kind: 'app' as const,
        clusterName: 'cluster',
        constraints: {
          aws_console: {
            role_arns: ['arn:1'],
          },
        },
      },
      'arn:1',
      set
    );
    expect(set).toHaveBeenCalledWith(
      'cluster/app/my-app' as ResourceIDString,
      undefined
    );
  });
});

describe('toggleSSHConstraint', () => {
  it('removes the specified login and keeps remaining', () => {
    const set = jest.fn<void, [ResourceIDString, ResourceConstraints?]>();
    toggleSSHConstraint(
      {
        id: 'my-node',
        kind: 'node' as const,
        clusterName: 'cluster',
        constraints: {
          ssh: { logins: ['root', 'ubuntu'] },
        },
      },
      'root',
      set
    );
    expect(set).toHaveBeenCalledWith(
      'cluster/node/my-node' as ResourceIDString,
      { ssh: { logins: ['ubuntu'] } }
    );
  });

  it('clears constraints when last login is removed', () => {
    const set = jest.fn<void, [ResourceIDString, ResourceConstraints?]>();
    toggleSSHConstraint(
      {
        id: 'my-node',
        kind: 'node' as const,
        clusterName: 'cluster',
        constraints: {
          ssh: { logins: ['root'] },
        },
      },
      'root',
      set
    );
    expect(set).toHaveBeenCalledWith(
      'cluster/node/my-node' as ResourceIDString,
      undefined
    );
  });
});
