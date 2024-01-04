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

import { displayDateTime } from 'shared/services/loc';

export function formatTabs(level) {
  const tabs = [];
  while (level > 1) {
    tabs.push('\t');
    level--;
  }

  return tabs.join('');
}

export function formatCmd(item) {
  const time = displayDateTime(item.time);
  const { args = '', program } = item;

  if (item.event === 'session.exec') {
    const [path, cmd] = splitProgramPath(program);
    return [time, path, cmd, args].join('\t');
  }

  return [time, '?', program].join('\t');
}

export function formatFile(item) {
  const { path } = item;
  return ['file', path].join('\t');
}

export function formatNetwork(item) {
  const { src, dest } = item;
  return ['network', src, '->', dest].join('\t');
}

export function splitProgramPath(program) {
  const temp = program.split('/');
  if (temp.length === 1) {
    return program;
  }

  program = temp.pop();
  return [temp.join('/'), program];
}
