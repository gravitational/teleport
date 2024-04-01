/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { render, screen, waitFor } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import TeleportContext from 'teleport/teleportContext';

import { AccountPage as Account } from 'teleport/Account/Account';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';

const defaultAuthType = cfg.auth.second_factor;
const defaultPasswordless = cfg.auth.allowPasswordless;

describe('passkey + mfa button state', () => {
  const ctx = createTeleportContext();

  beforeEach(() => {
    jest.spyOn(ctx.mfaService, 'fetchDevices').mockResolvedValue([]);
  });

  afterEach(() => {
    jest.clearAllMocks();
    cfg.auth.second_factor = defaultAuthType;
    cfg.auth.allowPasswordless = defaultPasswordless;
  });

  // Note: the "off" and "otp" cases don't make sense with passwordless turned
  // on (the auth server wouldn't start in this configuration), but we're still
  // testing them for completeness.
  test.each`
    mfa           | pwdless  | pkEnabled | mfaEnabled
    ${'on'}       | ${true}  | ${true}   | ${true}
    ${'on'}       | ${false} | ${false}  | ${true}
    ${'optional'} | ${true}  | ${true}   | ${true}
    ${'optional'} | ${false} | ${false}  | ${true}
    ${'otp'}      | ${false} | ${false}  | ${true}
    ${'otp'}      | ${true}  | ${true}   | ${true}
    ${'webauthn'} | ${true}  | ${true}   | ${true}
    ${'webauthn'} | ${false} | ${false}  | ${true}
    ${'off'}      | ${false} | ${false}  | ${false}
    ${'off'}      | ${true}  | ${true}   | ${false}
  `(
    '2fa($mfa) with pwdless($pwdless) = passkey($pkEnabled) mfa($mfaEnabled)',
    async ({ mfa, pwdless, pkEnabled, mfaEnabled }) => {
      cfg.auth.second_factor = mfa;
      cfg.auth.allowPasswordless = pwdless;

      renderComponent(ctx);

      await waitFor(() => {
        expect(screen.queryByTestId('indicator')).not.toBeInTheDocument();
      });

      // If btns are disabled, the disabled attr has a value of empty string.
      // If btns are not disabled, the disabled attr is null (not defined).

      // eslint-disable-next-line jest-dom/prefer-to-have-attribute
      expect(screen.getByText(/add a passkey/i).getAttribute('disabled')).toBe(
        pkEnabled ? null : ''
      );

      // eslint-disable-next-line jest-dom/prefer-to-have-attribute
      expect(screen.getByText(/add mfa/i).getAttribute('disabled')).toBe(
        mfaEnabled ? null : ''
      );
    }
  );
});

function renderComponent(ctx: TeleportContext) {
  return render(
    <ContextProvider ctx={ctx}>
      <Account />
    </ContextProvider>
  );
}
