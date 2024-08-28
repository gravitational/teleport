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

import React from 'react';

import { Banner } from 'design';

import { Action } from 'design/Alert';

import { CaptureEvent } from 'teleport/services/userEvent/types';
import { userEventService } from 'teleport/services/userEvent';

export type Severity = 'info' | 'warning' | 'danger';

type Props = {
  message: string;
  severity: Severity;
  id: string;
  link?: string;
  linkCTA?: string;
  onDismiss: () => void;
};

export function StandardBanner({
  id,
  message = '',
  severity = 'info',
  link = '',
  linkCTA = '',
  onDismiss,
}: Props) {
  const linkValid = isValidTeleportLink(link);
  const details = linkValid ? undefined : bannerDetails(link, linkCTA);
  const primaryAction = linkValid ? action(id, link, linkCTA) : undefined;

  return (
    <Banner
      kind={severity}
      details={details}
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

const bannerDetails = (link: string, cta: string): string =>
  cta ? `${cta}: ${link}` : link;

const action = (id: string, link: string, cta: string): Action => {
  return {
    content: cta || 'Learn More',
    href: link,
    onClick: () =>
      userEventService.captureUserEvent({
        event: CaptureEvent.BannerClickEvent,
        alert: id,
      }),
  };
};
