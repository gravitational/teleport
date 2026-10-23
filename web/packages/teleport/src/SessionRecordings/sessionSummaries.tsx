/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import { CtaEvent } from 'teleport/services/userEvent';

export const SESSION_SUMMARIES_DOCS_URL =
  'https://goteleport.com/docs/identity-security/session-summaries/';

// Above the sidebar resize border and the player; matches `HoverTooltip`.
export const PROMO_POPOVER_Z_INDEX = 1500;

export function UpgradeButton({ size }: { size: 'medium' | 'large' }) {
  return (
    <ButtonLockedFeature
      event={CtaEvent.CTA_IDENTITY_SECURITY}
      size={size}
      py={0}
    >
      {cfg.edition === 'ent' && cfg.isCloud
        ? 'Start a trial'
        : 'Unlock Session Summaries'}
    </ButtonLockedFeature>
  );
}
