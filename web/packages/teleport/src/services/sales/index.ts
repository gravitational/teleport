/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { CtaEvent } from 'teleport/services/userEvent';
import cfg from 'teleport/config';

// These URL's are the shorten URL version. These shorten URL
// are defined in the "next" repo.
const UPGRADE_TEAM_URL = 'https://goteleport.com/r/upgrade-team';
const UPGRADE_COMMUNITY_URL = 'https://goteleport.com/r/upgrade-community';
// UPGRADE_IGS_URL is enterprise upgrading to enterprise with Identity Governance & Security
const UPGRADE_IGS_URL = 'https://goteleport.com/r/upgrade-igs';

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
  event?: CtaEvent
) {
  let url = UPGRADE_COMMUNITY_URL;
  if (isEnterprise) {
    url = cfg.isTeam ? UPGRADE_TEAM_URL : UPGRADE_IGS_URL;
  }
  const params = getParams(version, isEnterprise, event);
  return `${url}?${params}`;
}
