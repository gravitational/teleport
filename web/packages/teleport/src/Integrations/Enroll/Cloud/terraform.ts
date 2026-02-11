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

/**
 * hcl is a tagged template function for generating Terraform HCL
 * configuration. Formats objects, arrays and primitives. If a null value is
 * used, it will omit the current line and preceding whitespace and single-line
 * comments.
 *
 * @example
 * const teleportAddr = "teleport.mycluster.example"
 * const clientIdList = ["discover.teleport"];
 * const tags         = null;
 *
 * const config = hcl`
 * resource "aws_iam_openid_connect_provider" "teleport_oidc_provider" {
 *   url             = ${teleportAddr}
 *   client_id_list  = ${clientIdList}
 *   tags            = ${tags}
 * }
 * `;
 *
 *
 * // Generates:
 * // resource "aws_iam_openid_connect_provider" "teleport_oidc_provider" {
 * //   url            = "teleport.mycluster.example"
 * //   client_id_list = ["discover.teleport"]
 * // }
 *
 * TODO(alexhemard): this covers a pretty narrow use case for formatting
 * simple Terraform module configuration examples. should look at moving this to the
 * backend and leveraging the official hcl go package for anything more sophisticated.
 **/
export const hcl = (
  strings: TemplateStringsArray,
  ...values: TFValue[]
): string => {
  let result = '';

  strings.forEach((str, i) => {
    if (i < values.length && values[i] === null) {
      // If null, remove current line, preceding whitespace, and single line comments
      const lines = str.split('\n').slice(0, -1);

      // find last non-whitespace, non-comment line
      const lastContentIndex = lines.findLastIndex(line => {
        const trimmed = line.trim();
        return trimmed !== '' && !trimmed.startsWith('#');
      });

      result += lines.slice(0, lastContentIndex + 1).join('\n');

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
        result += JSON.stringify(value);
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
    return renderObject(value, indent);
  }

  return '';
};

const renderArray = (value: TFValue[], indent: number): string => {
  if (value.length === 0) return '[]';
  if (value.every(v => typeof v !== 'object' || v === null)) {
    // render array as a single line if small enough
    const breakLength = 40;

    const singleLine =
      '[' + value.map(v => renderValue(v, indent + 1)).join(', ') + ']';
    if (singleLine.length <= breakLength) {
      return singleLine;
    }
  }

  const spaces = '  '.repeat(indent + 1);
  const items = value.map(v => renderValue(v, indent + 1));
  return `[\n${spaces}${items.join(`,\n${spaces}`)}\n${'  '.repeat(indent)}]`;
};

const renderObject = (value: TFObject, indent: number): string => {
  if (Object.keys(value).length === 0) return '{}';

  const maxLength = Math.max(
    ...Object.keys(value).map(k => renderKey(k).length)
  );

  const spaces = '  '.repeat(indent + 1);
  const entries = Object.entries(value).map(([key, value]) => {
    const padding = ' '.repeat(maxLength - renderKey(key).length);
    return `${renderKey(key)}${padding} = ${renderValue(value, indent + 1)}`;
  });
  return `{\n${spaces}${entries.join(`\n${spaces}`)}\n${'  '.repeat(indent)}}`;
};

const renderKey = (key: string): string => {
  // Source: https://developer.hashicorp.com/terraform/language/syntax/configuration#identifiers
  // Keys can be left unquoted if they are a valid identifier
  //
  // Identifiers can contain letters, digits, underscores (_), and hyphens
  // (-). The first character of an identifier must not be a digit, to avoid
  // ambiguity with literal numbers.

  const needsQuotes = !/^[a-zA-Z_][a-zA-Z0-9_-]*$/.test(key);
  return needsQuotes ? JSON.stringify(key) : key;
};
