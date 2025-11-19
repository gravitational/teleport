/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

interface TFObject {
  readonly [key: string]: TFValue;
}

type TFValue = string | number | boolean | TFValue[] | TFObject | null;

export const hcl = (
  strings: TemplateStringsArray,
  ...values: TFValue[]
): string => {
  let result = '';

  strings.forEach((str, i) => {
    if (i < values.length && values[i] === null) {
      return;
    }

    result += str;

    if (i < values.length) {
      const value = values[i];

      const lines = result.split('\n');
      const lastLine = lines[lines.length - 1];
      const currentIndent = lastLine.match(/^(\s*)/)?.[1] || '';
      const indentLevel = Math.floor(currentIndent.length / 2);

      if (typeof value === 'string') {
        if (value.includes('\n')) {
          const indentedValue = value
            .split('\n')
            .map((line, index) => {
              if (index === 0) return line;
              if (line.trim() === '') return '';
              return currentIndent + line;
            })
            .join('\n');
          result += indentedValue;
        } else {
          result += JSON.stringify(value);
        }
      } else {
        result += renderValue(value, indentLevel);
      }
    }
  });

  return result;
};

const renderValue = (value: TFValue, indent: number): string => {
  if (value === null) return '';

  // string, boolean, number
  if (typeof value !== 'object') return JSON.stringify(value);

  // TFValue[]
  if (Array.isArray(value)) {
    return renderArray(value, indent);
  }

  // TFObject
  if (typeof value === 'object') {
    if (Object.keys(value).length === 0) return '{}';

    const maxLength = Math.max(...Object.keys(value).map(a => a.length));
    const spaces = '  '.repeat(indent + 1);
    const entries = Object.entries(value).map(([key, value]) => {
      const padding = ' '.repeat(maxLength - key.length);
      return `${key}${padding} = ${renderValue(value, indent)}`;
    });
    return `{\n${spaces}${entries.join(`\n${spaces}`)}\n${'  '.repeat(indent)}}`;
  }

  return '';
};

const renderArray = (value: TFValue[], indent: number): string => {
  if (value.length === 0) return '[]';
  if (value.length === 1 && typeof value[0] !== 'object')
    return '[' + value.map(v => renderValue(v, indent + 1)) + ']';

  const spaces = '  '.repeat(indent + 1);
  const items = value.map(v => renderValue(v, indent + 1));
  return `[\n${spaces}${items.join(`,\n${spaces}`)}\n${'  '.repeat(indent)}]`;
};
