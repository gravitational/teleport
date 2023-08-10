import { CtaEvent } from 'teleport/services/userEvent';
const SALES_URL = 'https://goteleport.com/r/upgrade-team';

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
  const params = getParams(version, isEnterprise, event);
  return `${SALES_URL}?${params}`;
}
