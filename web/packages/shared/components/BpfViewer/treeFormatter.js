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
