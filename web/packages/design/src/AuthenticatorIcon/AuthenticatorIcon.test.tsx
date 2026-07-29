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

import { darkTheme, lightTheme, resolveTheme } from 'design/theme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { render, screen } from 'design/utils/testing';

import { AuthenticatorIcon } from './AuthenticatorIcon';

// Stand in for the generated map so the theme and fallback behavior can be tested against
// known AAGUIDs without pulling in every vendor asset.
jest.mock('./authenticatorSpecs', () => ({
  authenticatorSpecs: {
    'ee882879-721c-4913-9775-3dfcce97072a': {
      name: 'YubiKey 5 Series',
      light: 'yubikey-light.svg',
      dark: 'yubikey-dark.svg',
    },
    'aaaaaaaa-1111-1111-1111-111111111111': {
      name: 'Light only',
      light: 'light-only.png',
    },
    'bbbbbbbb-2222-2222-2222-222222222222': { name: 'No icons' },
  },
}));

const YUBIKEY = 'ee882879-721c-4913-9775-3dfcce97072a';

test('renders the light variant under the light theme', () => {
  renderIcon(YUBIKEY, { type: 'light' });

  expect(screen.getByTestId(`authenticator-icon-${YUBIKEY}`)).toHaveAttribute(
    'src',
    'yubikey-light.svg'
  );
});

test('renders the dark variant under the dark theme', () => {
  renderIcon(YUBIKEY, { type: 'dark' });

  expect(screen.getByTestId(`authenticator-icon-${YUBIKEY}`)).toHaveAttribute(
    'src',
    'yubikey-dark.svg'
  );
});

test('falls back to the other variant when a theme has no artwork', () => {
  const aaguid = 'aaaaaaaa-1111-1111-1111-111111111111';
  renderIcon(aaguid, { type: 'dark' });

  expect(screen.getByTestId(`authenticator-icon-${aaguid}`)).toHaveAttribute(
    'src',
    'light-only.png'
  );
});

test('sizes the image', () => {
  renderIcon(YUBIKEY, { size: 48 });

  const img = screen.getByTestId(`authenticator-icon-${YUBIKEY}`);

  expect(img).toHaveAttribute('width', '48');
  expect(img).toHaveAttribute('height', '48');
});

test.each([
  { name: 'an unknown AAGUID', aaguid: 'ffffffff-0000-0000-0000-000000000000' },
  {
    name: 'a known AAGUID with no icons',
    aaguid: 'bbbbbbbb-2222-2222-2222-222222222222',
  },
  { name: 'a missing AAGUID', aaguid: undefined },
  // An AAGUID naming an inherited property must not resolve through Object.prototype.
  { name: 'an inherited property name', aaguid: 'constructor' },
])('renders the generic key icon for $name', ({ aaguid }) => {
  renderIcon(aaguid);
  expect(
    screen.queryByTestId(`authenticator-icon-${aaguid}`)
  ).not.toBeInTheDocument();
  expect(screen.getByTestId('authenticator-icon-fallback')).toBeInTheDocument();
});

function renderIcon(
  aaguid: string | undefined,
  { type = 'dark', size }: { type?: string; size?: number } = {}
) {
  return render(
    <ConfiguredThemeProvider
      theme={resolveTheme(type === 'light' ? lightTheme : darkTheme)}
    >
      <AuthenticatorIcon aaguid={aaguid} size={size} />
    </ConfiguredThemeProvider>
  );
}
