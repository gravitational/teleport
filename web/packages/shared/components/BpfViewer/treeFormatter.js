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

import { formatTabs, formatCmd, formatNetwork, formatFile } from './formatters';

export default function formatTree(tree, events, buffer) {
  if (!tree) {
    return;
  }

  const tabStr = formatTabs(tree.level);

  if (tree.index !== -1) {
    const processStr = formatCmd(events[tree.index]);
    buffer.push(`${tabStr}${processStr}`);
  }

  Object.keys(tree.children).forEach(kid => {
    formatTree(tree.children[kid], events, buffer);
  });

  if (tree.index !== -1) {
    tree.network.map(fIndex => {
      const fStr = formatNetwork(events[fIndex]);
      buffer.push(`\t${tabStr}${fStr}`);
    });

    tree.files.map(fIndex => {
      const fStr = formatFile(events[fIndex]);
      buffer.push(`\t${tabStr}${fStr}`);
    });
  }
}
