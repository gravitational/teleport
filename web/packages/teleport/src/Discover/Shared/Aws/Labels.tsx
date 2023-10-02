/**
 * Copyright 2023 Gravitational, Inc.
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

import React from 'react';

import { Flex, Label as Pill } from 'design';

import { Label } from 'teleport/types';

export const Labels = ({ labels }: { labels: Label[] }) => {
  const $labels = labels.map((label, index) => {
    const labelText = `${label.name}: ${label.value}`;

    return (
      <Pill key={`${label.name}${label.value}${index}`} mr="1" kind="secondary">
        {labelText}
      </Pill>
    );
  });

  return <Flex flexWrap="wrap">{$labels}</Flex>;
};

// labelMatcher allows user to client search by labels in the format
//   1) `key: value` or
//   2) `key:value` or
//   3) `key` or `value`
export function labelMatcher<T>(
  targetValue: any,
  searchValue: string,
  propName: keyof T & string
) {
  if (propName === 'labels') {
    return targetValue.some((label: Label) => {
      const convertedKey = label.name.toLocaleUpperCase();
      const convertedVal = label.value.toLocaleUpperCase();
      const formattedWords = [
        `${convertedKey}:${convertedVal}`,
        `${convertedKey}: ${convertedVal}`,
      ];
      return formattedWords.some(w => w.includes(searchValue));
    });
  }
}
