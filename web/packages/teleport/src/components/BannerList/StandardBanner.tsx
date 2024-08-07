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

import { Banner, Link } from 'design';

import { CaptureEvent } from 'teleport/services/userEvent/types';
import { userEventService } from 'teleport/services/userEvent';

export type Severity = 'info' | 'warning' | 'danger';

export type Props = {
  message: string;
  severity: Severity;
  id: string;
  link?: string;
  onDismiss: (id: string) => void;
};

export function StandardBanner({
  id,
  message = '',
  severity = 'info',
  link = '',
  onDismiss,
}: Props) {
  const isValidTeleportLink = (link: string) => {
    try {
      const url = new URL(link);
      return url.hostname === 'goteleport.com';
    } catch {
      return false;
    }
  };

  return (
    <Banner kind={severity} onDismiss={() => onDismiss(id)} dismissible>
      {isValidTeleportLink(link) ? (
        <Link
          href={link}
          target="_blank"
          css={{ fontWeight: 'inherit' }}
          onClick={() =>
            userEventService.captureUserEvent({
              event: CaptureEvent.BannerClickEvent,
              alert: id,
            })
          }
        >
          {message}
        </Link>
      ) : (
        message
      )}
    </Banner>
  );
}
