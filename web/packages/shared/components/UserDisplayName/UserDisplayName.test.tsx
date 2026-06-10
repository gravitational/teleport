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

import { render, screen, userEvent, within } from 'design/utils/testing';

import { UserDisplayName, type UserDisplayNameLayout } from './UserDisplayName';

describe('UserDisplayName', () => {
  const username = 'alice@example.com';
  const layouts: UserDisplayNameLayout[] = ['inline', 'stacked', 'tooltip'];
  const valueScenarios: {
    name: string;
    primaryText?: string;
    secondaryText?: string;
    expectedPrimary: string;
    expectedSecondary: string | null;
    expectedVisibleUsernameCountByLayout: Record<UserDisplayNameLayout, number>;
  }[] = [
    {
      name: 'primary, secondary, and username',
      primaryText: 'Alice Jones',
      secondaryText: 'Engineering',
      expectedPrimary: 'Alice Jones',
      expectedSecondary: 'Engineering',
      expectedVisibleUsernameCountByLayout: {
        inline: 1,
        stacked: 1,
        tooltip: 0,
      },
    },
    {
      name: 'only username',
      expectedPrimary: username,
      expectedSecondary: null,
      expectedVisibleUsernameCountByLayout: {
        inline: 1,
        stacked: 1,
        tooltip: 1,
      },
    },
    {
      name: 'primary and username',
      primaryText: 'Alice Jones',
      expectedPrimary: 'Alice Jones',
      expectedSecondary: null,
      expectedVisibleUsernameCountByLayout: {
        inline: 1,
        stacked: 1,
        tooltip: 0,
      },
    },
    {
      name: 'secondary and username',
      secondaryText: 'Engineering',
      expectedPrimary: username,
      expectedSecondary: 'Engineering',
      expectedVisibleUsernameCountByLayout: {
        inline: 1,
        stacked: 1,
        tooltip: 1,
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
          scenario.expectedSecondary ? 1 : 0
        );

        const expectedVisibleUsernameCount =
          scenario.expectedVisibleUsernameCountByLayout[layout];
        expect(screen.queryAllByText(username)).toHaveLength(
          expectedVisibleUsernameCount
        );

        const tooltipTriggerLabel = getTooltipAriaLabel(
          scenario.expectedPrimary,
          scenario.expectedSecondary,
          username
        );
        expect(screen.queryAllByLabelText(tooltipTriggerLabel)).toHaveLength(
          layout === 'tooltip' && expectedVisibleUsernameCount === 0 ? 1 : 0
        );
      });
    }
  }

  it('formats inline supporting values with delimiters', () => {
    render(
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
    const inlineSecondary = within(inlineSupportingValues).getByText(
      'Engineering'
    );

    expect(primaryLine).toContainElement(inlineSupportingValues);
    expect(inlineSupportingValues).toHaveStyleRule('content', "'('", {
      modifier: '::before',
    });
    expect(inlineSupportingValues).toHaveStyleRule('content', "')'", {
      modifier: '::after',
    });
    expect(inlineSecondary).toHaveStyleRule('content', "'•'", {
      modifier: '::before',
    });
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

  it('defaults to tooltip layout', async () => {
    const user = userEvent.setup();

    render(
      <UserDisplayName
        username={username}
        primaryText="Alice Jones"
        secondaryText="Engineering"
      />
    );

    expect(screen.queryByText(username)).not.toBeInTheDocument();

    await user.hover(screen.getByText('Alice Jones'));

    const tooltip = await screen.findByRole('tooltip');
    expect(within(tooltip).getByText(username)).toBeInTheDocument();
  });

  it('anchors tooltip layout to the primary text', () => {
    render(
      <UserDisplayName
        username={username}
        primaryText="Alice Jones"
        secondaryText="Engineering"
        layout="tooltip"
      />
    );

    const tooltipTrigger = screen.getByLabelText(
      'Alice Jones, Engineering, username alice@example.com'
    );
    expect(tooltipTrigger).toBe(screen.getByText('Alice Jones'));
  });

  function getTooltipAriaLabel(
    primary: string,
    secondary: string | null | undefined,
    username: string
  ) {
    return [primary, secondary, `username ${username}`]
      .filter(Boolean)
      .join(', ');
  }
});
