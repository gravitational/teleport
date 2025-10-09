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

import { CtaEvent } from 'teleport/services/userEvent';

// These URLs are the shorten URL version. These marketing URL's
// are defined in the "next" repo.
// eg: https://github.com/gravitational/next/pull/2298
const UPGRADE_COMMUNITY_URL = 'https://goteleport.com/r/upgrade-community';
// UPGRADE_IGS_URL is enterprise upgrading to enterprise with Identity Governance & Security
const UPGRADE_IGS_URL = 'https://goteleport.com/r/upgrade-igs';
// UPGRADE_POLICY_URL is enterprise upgrading to enterprise with Policy
export const UPGRADE_POLICY_URL = 'https://goteleport.com/r/upgrade-policy';

function getParams(
  version: string,
  isEnterprise: boolean,
  event?: CtaEvent
): string {
  return `${isEnterprise ? 'e_' : ''}${version}&utm_campaign=${
    CtaEvent[event ?? CtaEvent.CTA_UNSPECIFIED]
  }`;
}

export function getSalesURL(
  version: string,
  isEnterprise: boolean,
  event?: CtaEvent,
  url?: string
) {
  if (!url) {
    url = isEnterprise ? UPGRADE_IGS_URL : UPGRADE_COMMUNITY_URL;
  }
  const params = getParams(version, isEnterprise, event);
  return `${url}?${params}`;
}
