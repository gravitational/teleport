/**
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
