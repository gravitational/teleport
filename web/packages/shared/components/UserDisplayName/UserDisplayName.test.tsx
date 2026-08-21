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

import { render, screen, within } from 'design/utils/testing';

import { UserDisplayName, type UserDisplayNameLayout } from './UserDisplayName';

describe('UserDisplayName', () => {
  const username = 'alice@example.com';
  const layouts: UserDisplayNameLayout[] = ['inline', 'stacked'];
  const valueScenarios: {
    name: string;
    primaryText?: string;
    secondaryText?: string;
    expectedPrimary: string;
    expectedVisibleSecondaryCountByLayout: Record<
      UserDisplayNameLayout,
      number
    >;
  }[] = [
    {
      name: 'primary, secondary, and username',
      primaryText: 'Alice Jones',
      secondaryText: 'Engineering',
      expectedPrimary: 'Alice Jones',
      expectedVisibleSecondaryCountByLayout: {
        inline: 0,
        stacked: 1,
      },
    },
    {
      name: 'only username',
      expectedPrimary: username,
      expectedVisibleSecondaryCountByLayout: {
        inline: 0,
        stacked: 0,
      },
    },
    {
      name: 'primary and username',
      primaryText: 'Alice Jones',
      expectedPrimary: 'Alice Jones',
      expectedVisibleSecondaryCountByLayout: {
        inline: 0,
        stacked: 0,
      },
    },
    {
      name: 'secondary and username',
      secondaryText: 'Engineering',
      expectedPrimary: username,
      expectedVisibleSecondaryCountByLayout: {
        inline: 0,
        stacked: 1,
      },
    },
  ];

  for (const scenario of valueScenarios) {
    for (const layout of layouts) {
      it(`renders ${scenario.name} with ${layout} layout`, () => {
        render(
          <UserDisplayName
            username={username}
            primaryText={scenario.primaryText}
            secondaryText={scenario.secondaryText}
            layout={layout}
          />
        );

        expect(screen.getByText(scenario.expectedPrimary)).toBeInTheDocument();
        expect(screen.queryAllByText('Engineering')).toHaveLength(
          scenario.expectedVisibleSecondaryCountByLayout[layout]
        );

        expect(screen.queryAllByText(username)).toHaveLength(1);
      });
    }
  }

  it('ignores secondary text in inline layout', () => {
    const { container, rerender } = render(
      <UserDisplayName
        username={username}
        primaryText="Alice Jones"
        secondaryText="Engineering"
        layout="inline"
      />
    );

    const primaryLine = screen.getByText('Alice Jones')
      .parentElement as HTMLElement;
    const inlineUsername = within(primaryLine).getByText(username);
    const inlineSupportingValues = inlineUsername.parentElement as HTMLElement;

    expect(primaryLine).toContainElement(inlineSupportingValues);
    expect(inlineSupportingValues).toHaveStyleRule('content', "'('", {
      modifier: '::before',
    });
    expect(inlineSupportingValues).toHaveStyleRule('content', "')'", {
      modifier: '::after',
    });
    expect(screen.queryByText('Engineering')).not.toBeInTheDocument();

    const withSecondaryText = container.innerHTML;
    rerender(
      <UserDisplayName
        username={username}
        primaryText="Alice Jones"
        layout="inline"
      />
    );
    expect(container.innerHTML).toBe(withSecondaryText);
  });

  it('renders stacked supporting values together below the primary line', () => {
    render(
      <UserDisplayName
        username={username}
        primaryText="Alice Jones"
        secondaryText="Engineering"
        layout="stacked"
      />
    );

    const primaryLine = screen.getByText('Alice Jones')
      .parentElement as HTMLElement;
    expect(
      within(primaryLine).queryByText('Engineering')
    ).not.toBeInTheDocument();
    expect(within(primaryLine).queryByText(username)).not.toBeInTheDocument();

    const supportingLine = screen.getByText(username)
      .parentElement as HTMLElement;
    const secondary = within(supportingLine).getByText('Engineering');

    expect(supportingLine).toContainElement(screen.getByText(username));
    expect(secondary).toHaveStyleRule('content', "'•'", {
      modifier: '::before',
    });
  });

  it('does not repeat the username when primary text is absent', () => {
    render(
      <UserDisplayName
        username={username}
        secondaryText="Engineering"
        layout="stacked"
      />
    );

    expect(screen.queryAllByText(username)).toHaveLength(1);

    const primaryLine = screen.getByText(username).parentElement as HTMLElement;
    expect(
      within(primaryLine).queryByText('Engineering')
    ).not.toBeInTheDocument();
    expect(screen.getByText('Engineering')).toBeInTheDocument();
  });
});
