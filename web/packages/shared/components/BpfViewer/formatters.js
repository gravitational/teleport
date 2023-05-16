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
