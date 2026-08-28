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

import { screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import ButtonSso from '.';

test.each`
  ssoType        | expectedIcon
  ${'Microsoft'} | ${'res-icon-microsoft'}
  ${'github'}    | ${'res-icon-github'}
  ${'bitbucket'} | ${'res-icon-atlassianbitbucket'}
  ${'google'}    | ${'res-icon-google'}
`('rendering of $ssoType', ({ ssoType, expectedIcon }) => {
  render(<ButtonSso ssoType={ssoType} title="hello" />);

  expect(screen.getByRole('img')).toHaveAttribute('data-testid', expectedIcon);
  expect(screen.getByText(/hello/i)).toBeInTheDocument();
});

test('rendering unknown SSO type', () => {
  render(<ButtonSso ssoType="unknown" title="hello" />);

  expect(screen.getByTestId('icon')).toHaveClass('icon-key');
  expect(screen.getByText(/hello/i)).toBeInTheDocument();
});
