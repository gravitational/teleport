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

import { Flex } from 'design';
import TextEditor from 'shared/components/TextEditor';

type TerraformAwsIamProps = {
  integrationName: string;
  accountId: string;
  regions: string[];
  ec2Enabled?: boolean;
  ec2Matchers?: Record<string, string>[];
  rdsEnabled?: boolean;
  rdsMatchers?: Record<'name' | 'value', string>[];
  eksEnabled?: boolean;
  eksMatchers?: Record<'name' | 'value', string>[];
};

export function TerraformAwsIam(props: TerraformAwsIamProps) {
  return (
    <Flex height="600px" width="100%" m={0} bg="levels.deep">
      <TextEditor
        bg="levels.deep"
        data={[
          {
            content: makeTerraform(props),
            type: 'terraform',
          },
        ]}
        copyButton
        readOnly
      />
    </Flex>
  );
}

const makeTerraform = (ctx: {
  integrationName?: string;
  accountId?: string;
  regions?: string[];
  ec2Enabled?: boolean;
  ec2Matchers?: Record<'name' | 'value', string>[];
  rdsEnabled?: boolean;
  rdsMatchers?: Record<'name' | 'value', string>[];
  eksEnabled?: boolean;
  eksMatchers?: Record<'name' | 'value', string>[];
}) => {
  const integrationName = ctx.integrationName?.trim() || '<integration_name>';
  const accountId = ctx.accountId?.trim() || '<account_id>';
  const regions = ctx.regions.length > 0 ? ctx.regions.sort() : null;
  const ec2Enabled = ctx.ec2Enabled || null;
  const rdsEnabled = ctx.rdsEnabled || null;
  const eksEnabled = ctx.eksEnabled || null;

  const filteredEc2Matchers = ctx.ec2Matchers.filter(
    o => !!o['value'] || !!o['name']
  );

  const filteredRdsMatchers = ctx.rdsMatchers.filter(
    o => !!o['value'] || !!o['name']
  );

  const filteredEksMatchers = ctx.eksMatchers.filter(
    o => !!o['value'] || !!o['name']
  );

  const ec2Matchers =
    ec2Enabled && filteredEc2Matchers?.length > 0 ? filteredEc2Matchers : null;
  const rdsMatchers =
    rdsEnabled && filteredRdsMatchers?.length > 0 ? filteredRdsMatchers : null;
  const eksMatchers =
    eksEnabled && filteredEksMatchers?.length > 0 ? filteredEksMatchers : null;

  const tfModule = hcl`# Terraform Module
module "teleport-aws-discovery-${raw(integrationName)}" {
  source = "github.com/gravitational/teleport.git/examples/modules/whatever"

  integration_name = ${integrationName}
  aws_account_id   = ${parseInt(accountId) || null}

  regions = ${regions}

  ec2_enabled = ${ec2Enabled}

  ec2_matchers = ${ec2Matchers}

  rds_enabled = ${rdsEnabled}

  rds_matchers = ${rdsMatchers}

  eks_enabled = ${eksEnabled}

  eks_matchers = ${eksMatchers}
}`;

  return tfModule;
};

interface TFObject {
  [key: string]: TFValue;
}

type TFValue = string | number | boolean | TFValue[] | TFObject | null;

interface TFAlignedAttrs {
  type: 'aligned-attrs';
  attrs: TFAttr[];
}

interface TFAttr {
  type: 'attr';
  name: string;
  value: TFValue;
}

interface TFBlock {
  type: 'block';
  blockType: string;
  labels?: string[];
  body: TFNode[];
}

interface TFSnippet {
  type: 'snippet';
  value: string;
}

type TFNode = TFAttr | TFBlock | TFSnippet | TFAlignedAttrs;

const attr = (name: string, value: TFValue): TFAttr => ({
  type: 'attr',
  name,
  value,
});

const block = (
  blockType: 'resource' | 'module',
  labels: string[],
  ...body: TFNode[]
): TFBlock => ({
  type: 'block',
  blockType,
  labels,
  body,
});

const module =
  (name: string) =>
  (...body: TFNode[]): TFBlock =>
    block('module', [name], ...body);

const resource =
  (resourceName: string, name: string) =>
  (...body: TFNode[]): TFBlock =>
    block('resource', [resourceName, name], ...body);

const snippet = (value: string): TFSnippet => {
  return { type: 'snippet', value: value };
};

const alignAttrs = (...attrs: TFAttr[]): TFAlignedAttrs => ({
  type: 'aligned-attrs',
  attrs,
});

const render = (nodes: TFNode[], indent = 0): string => {
  const spaces = '  '.repeat(indent);
  const parts: string[] = [];

  nodes.forEach(node => {
    if (node === null) {
      return;
    }
    if (node.type === 'snippet') {
      const indented = node.value
        .split('\n')
        .map(line => (line.trim() === '' ? '' : `${spaces}${line}`))
        .join('\n');
      parts.push(indented);
    }
    if (node.type === 'attr') {
      if (node.value === null) {
        return;
      }

      parts.push(`${spaces}${node.name} = ${renderValue(node.value, indent)}`);
    }
    if (node.type === 'aligned-attrs') {
      const maxLength = Math.max(...node.attrs.map(a => a.name.length));
      const aligned = node.attrs
        .map(a => {
          const padding = ' '.repeat(maxLength - a.name.length);
          return `${spaces}${a.name}${padding} = ${renderValue(a.value, indent)}`;
        })
        .join('\n');
      parts.push(aligned);
    }
    if (node.type === 'block') {
      const labels = node.labels?.map(l => `"${l}"`).join(' ') || '';
      const header = `${spaces}${node.blockType}${labels ? ' ' + labels : ''} {`;
      const body = render(node.body, indent + 1);
      const footer = `${spaces}}`;
      parts.push([header, body, footer].filter(Boolean).join('\n'));
    }
  });

  return parts.join('\n\n');
};

class RawString {
  constructor(public value: string) {}
}

const raw = (value: string) => new RawString(value);

const hcl = (strings: TemplateStringsArray, ...values: any[]): string => {
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
  if (value === null) return ``;

  if (value instanceof RawString) {
    return value.value;
  }

  if (typeof value !== 'object') return JSON.stringify(value);

  if (Array.isArray(value)) {
    if (value.length === 0) return '[]';

    const spaces = '  '.repeat(indent + 1);
    const items = value.map(v => renderValue(v, indent + 1));
    return `[\n${spaces}${items.join(`,\n${spaces}`)}\n${'  '.repeat(indent)}]`;
  }

  if (typeof value === 'object' && !Array.isArray(value)) {
    if (value.length === 0) return '{}';

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
