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

import { Banner } from 'design';
import { Action } from 'design/Alert';

import { userEventService } from 'teleport/services/userEvent';
import { CaptureEvent } from 'teleport/services/userEvent/types';

export type Severity = 'info' | 'warning' | 'danger';

type Props = {
  message: string;
  severity: Severity;
  id: string;
  link?: string;
  linkText?: string;
  onDismiss: () => void;
};

export function StandardBanner({
  id,
  message = '',
  severity = 'info',
  link = '',
  linkText = '',
  onDismiss,
}: Props) {
  let primaryAction: Action | undefined;
  let invalidLinkFallback: string | undefined;

  // We want to only use the provided link if it's valid (that is, when it
  // doesn't parse or it's from outside Teleport domain). Otherwise, we display
  // it as plain text.
  if (isValidTeleportLink(link)) {
    primaryAction = {
      content: linkText || 'Learn More',
      href: link,
      onClick: () =>
        userEventService.captureUserEvent({
          event: CaptureEvent.BannerClickEvent,
          alert: id,
        }),
    };
  } else {
    invalidLinkFallback = linkText ? `${linkText}: ${link}` : link;
  }

  return (
    <Banner
      kind={severity}
      details={invalidLinkFallback}
      primaryAction={primaryAction}
      onDismiss={onDismiss}
      dismissible
    >
      {message}
    </Banner>
  );
}

const isValidTeleportLink = (link: string) => {
  try {
    const url = new URL(link);
    return url.hostname === 'goteleport.com';
  } catch {
    return false;
  }
};
