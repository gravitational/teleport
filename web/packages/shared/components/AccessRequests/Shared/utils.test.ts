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
  getFormattedDurationTxt,
  toggleAWSConsoleConstraint,
  toggleSSHConstraint,
} from './utils';

describe('getFormattedDurationTxt', () => {
  const start = new Date('2026-10-08T09:00:00Z');

  test.each`
    name                      | end                       | long                          | compact
    ${'whole days'}           | ${'2026-10-16T09:00:00Z'} | ${'8 days'}                   | ${'8d'}
    ${'days and hours'}       | ${'2026-10-16T10:00:00Z'} | ${'8 days 1 hour'}            | ${'8d 1h'}
    ${'days, hours, minutes'} | ${'2026-10-16T10:30:00Z'} | ${'8 days 1 hour 30 minutes'} | ${'8d 1h 30m'}
    ${'hours short of a day'} | ${'2026-10-16T07:00:00Z'} | ${'7 days 22 hours'}          | ${'7d 22h'}
    ${'minutes only'}         | ${'2026-10-08T09:30:00Z'} | ${'30 minutes'}               | ${'30m'}
    ${'no duration'}          | ${'2026-10-08T09:00:00Z'} | ${''}                         | ${''}
  `(
    'formats $name',
    ({
      end,
      long,
      compact,
    }: {
      end: string;
      long: string;
      compact: string;
    }) => {
      // Set up a duration bounded by the shared start and the case's end.
      const range = { start, end: new Date(end) };

      // Assert that omitting the flag keeps the spelled out units.
      expect(getFormattedDurationTxt(range)).toBe(long);

      // Assert that the compact flag shortens units to their initials.
      expect(getFormattedDurationTxt({ ...range, compact: true })).toBe(
        compact
      );
    }
  );
});

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
