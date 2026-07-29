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

import { useTheme } from 'styled-components';

import * as Icon from '../Icon';
import { authenticatorLogo } from './authenticator';

interface AuthenticatorIconProps {
  /** The authenticator's AAGUID. Unknown or missing AAGUIDs render a generic key icon. */
  aaguid?: string;
  /** Pixel size of the rendered icon. */
  size?: number;
}

/**
 * Displays the vendor icon for an authenticator identified by its AAGUID, picking the light or dark
 * variant for the current theme. Falls back to a generic key icon when the AAGUID is unknown or the
 * vendor provides no icon.
 */
export function AuthenticatorIcon({
  aaguid,
  size = 24,
}: AuthenticatorIconProps) {
  const theme = useTheme();
  const logo = authenticatorLogo(aaguid);
  const src =
    theme.type === 'dark'
      ? (logo?.dark ?? logo?.light)
      : (logo?.light ?? logo?.dark);

  if (!src) {
    return <Icon.Key size={size} data-testid="authenticator-icon-fallback" />;
  }

  return (
    <img
      src={src}
      width={size}
      height={size}
      alt=""
      data-testid={`authenticator-icon-${aaguid}`}
    />
  );
}
