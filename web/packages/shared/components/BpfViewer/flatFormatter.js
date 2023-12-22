/*
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

import { formatCmd, formatNetwork, formatFile } from './formatters';

export default function flatFormatter(events, buffer) {
  for (var i = 0; i < events.length; i++) {
    const event = events[i];

    if (event.event === 'session.exec') {
      buffer.push(formatCmd(event));
    } else if (event.event === 'session.connect') {
      buffer.push(formatNetwork(event));
    } else if (event.event === 'session.open') {
      buffer.push(formatFile(event));
    }
  }
}
