/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
