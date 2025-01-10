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

import { useLocation } from 'react-router';

import { LogoutFailed } from 'design/CardError';

import { LogoHero } from 'teleport/components/LogoHero';
import cfg from 'teleport/config';

export function SingleLogoutFailed() {
  const { search } = useLocation();
  const params = new URLSearchParams(search);
  const connectorName = params.get('connectorName');

  const connectorNameText = connectorName || 'your SAML identity provider';
  return (
    <>
      <LogoHero />
      <LogoutFailed
        loginUrl={cfg.routes.login}
        message={`You have been logged out of Teleport, but we were unable to log you out of ${connectorNameText}. See the Teleport logs for details.`}
      />
    </>
  );
}
