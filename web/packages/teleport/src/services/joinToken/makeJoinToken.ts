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

import { formatDistanceStrict } from 'date-fns';

import type { JoinToken } from './types';

export const INTERNAL_RESOURCE_ID_LABEL_KEY = 'teleport.internal/resource-id';

export default function makeToken(json): JoinToken {
  json = json || {};
  const { id, expiry, suggestedLabels } = json;

  const labels = suggestedLabels || [];

  return {
    id,
    suggestedLabels: labels,
    internalResourceId: extractInternalResourceId(labels),
    expiry: expiry ? new Date(expiry) : null,
    expiryText: expiry
      ? formatDistanceStrict(new Date(), new Date(expiry))
      : '',
  };
}

function extractInternalResourceId(labels: any[]) {
  let resourceId = '';
  labels.forEach(l => {
    if (l.name === INTERNAL_RESOURCE_ID_LABEL_KEY) {
      resourceId = l.value;
    }
  });

  return resourceId;
}
