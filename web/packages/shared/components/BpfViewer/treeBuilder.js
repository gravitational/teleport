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

export default function makeTree(events) {
  const processMap = new Map();
  const tree = {
    pid: 'root',
    children: {},
    index: -1,
    level: 0,
  };

  // create a lookup table
  events.forEach((s, index) => {
    if (!processMap.has(s.pid)) {
      processMap.set(s.pid, {
        files: [],
        network: [],
        ppid: s.ppid,
        index,
      });
    }

    const process = processMap.get(s.pid);

    if (s.event === 'session.connect') {
      process.network.push(index);
    }

    if (s.event === 'session.open') {
      process.files.push(index);
    }

    if (s.event === 'session.exec') {
      process.ppid = s.ppid;
      process.index = index;
    }
  });

  // build a tree
  processMap.forEach((process, keyPid) => {
    const ppath = [keyPid];
    let pid = process.ppid;
    while (processMap.has(pid)) {
      ppath.unshift(pid);
      pid = processMap.get(pid).ppid;
    }

    let current = tree.children;
    const level = ppath.length - 1;
    for (var i = 0; i < ppath.length; i++) {
      const pid = ppath[i];
      current[pid] = current[pid] || {
        index: processMap.get(pid).index,
        pid,
        level,
        files: processMap.get(pid).files,
        network: processMap.get(pid).network,
        children: {},
      };

      current = current[pid].children;
    }
  });

  return tree;
}
