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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

/**
 * reportClientError reports a client-side web UI error to this cluster's
 * proxy, so it shows up in the proxy's own logs.
 * - `component` identifies which part of the UI is reporting
 * - `message` is the error text
 * - `metadata` is optional additional context (e.g. retry count, cluster version)
 */
export function reportClientError(
  component: string,
  message: string,
  metadata?: Record<string, string>
) {
  void api.fetch(cfg.api.logClientErrorPath, {
    method: 'POST',
    body: JSON.stringify({ component, message, metadata }),
  });
}
