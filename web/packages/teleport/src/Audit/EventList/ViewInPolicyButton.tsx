/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useMemo } from 'react';
import { NavLink } from 'react-router-dom';

import { ButtonBorder } from 'design';

import cfg from 'teleport/config';
import {
  Event,
  EventCode,
  eventCodes,
  RawEvents,
} from 'teleport/services/audit';

const VIEW_IN_POLICY_EVENTS: EventCode[] = [
  eventCodes.ACCESS_GRAPH_PATH_CHANGED,
];

function getUrlForEvent<C extends EventCode>(event: RawEvents[C]) {
  switch (event.code) {
    case eventCodes.ACCESS_GRAPH_PATH_CHANGED:
      return cfg.getAccessGraphCrownJewelAccessPathUrl(event.change_id);
  }

  // eslint-disable-next-line no-console
  console.warn(
    'Unsupported event code for "View in Policy" button',
    event.code
  );
}

export function ViewInPolicyButton({ event }: { event: Event }) {
  const shouldShow = VIEW_IN_POLICY_EVENTS.includes(event.code);

  const url = useMemo(
    () => shouldShow && getUrlForEvent(event.raw),
    [shouldShow, event]
  );

  if (!shouldShow || !url) {
    return null;
  }

  return (
    <ButtonBorder
      as={NavLink}
      to={url}
      size="small"
      style={{ whiteSpace: 'nowrap' }}
    >
      View in Policy
    </ButtonBorder>
  );
}
