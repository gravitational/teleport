/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useMemo } from 'react';
import { makeLabelTag } from 'teleport/components/formatters';
import { Label } from 'teleport/types';

export default function useLabelOptions<T extends Data>(
  data: T[],
  labels: Label[]
) {
  const all = useMemo<Option[]>(() => makeOptionsFromData(data), [data]);
  const selected = useMemo<Option[]>(() => makeOptions(labels), [labels]);

  return {
    all,
    selected,
  };
}

function makeOptionsFromData<T extends Data>(data: T[] = []): Option[] {
  // Test a tags and labels field exist.
  if (!data.length || !data[0].labels) {
    return [];
  }

  // Extract unique labels.
  const tagDict = {};
  data.forEach(({ labels }) => {
    labels.forEach(label => {
      const tag = makeLabelTag(label);
      if (!tagDict[tag]) {
        tagDict[tag] = label;
      }
    });
  });

  return Object.keys(tagDict)
    .sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))
    .map(tag => ({ label: tag, value: tag, obj: tagDict[tag] }));
}

function makeOptions(labels: Label[] = []): Option[] {
  return labels.map(label => {
    const tag = makeLabelTag(label);
    return { label: tag, value: tag, obj: label };
  });
}

export type Option = {
  value: string;
  label: string;

  // obj preserves original data
  obj: Label;
};

export type Data = {
  labels: Label[];
};

export type State = ReturnType<typeof useLabelOptions>;
