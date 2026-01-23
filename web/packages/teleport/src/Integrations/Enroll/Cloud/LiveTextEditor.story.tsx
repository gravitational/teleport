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

import { Meta } from '@storybook/react-vite';

import Flex from 'design/Flex';

import LiveTextEditor from './LiveTextEditor';
import { hcl } from './terraform';

type RegionCount = 'none' | '1' | '2' | '5';
type TagsOption = 'none' | 'many';

type StoryProps = {
  cluster_name: string;
  region_count: RegionCount;
  is_enabled: boolean;
  tags: TagsOption;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Integrations/LiveTextEditor',
  component: TerraformModule,
  argTypes: {
    cluster_name: {
      control: { type: 'text' },
    },
    region_count: {
      control: { type: 'select' },
      options: ['none', '1', '2', '5'],
    },
    is_enabled: {
      control: { type: 'boolean' },
    },
    tags: {
      control: { type: 'select' },
      options: ['none', 'many'],
    },
  },
  args: {
    cluster_name: 'teleport-cluster',
    region_count: '2',
    is_enabled: false,
    tags: 'none',
  },
};
export default meta;

const getRegions = (regionCount: RegionCount): string[] | null => {
  switch (regionCount) {
    case 'none':
      return null;
    case '1':
      return ['us-east-1'];
    case '2':
      return ['us-east-1', 'us-west-2'];
    case '5':
      return [
        'us-east-1',
        'us-west-2',
        'eu-west-1',
        'ap-southeast-1',
        'ca-central-1',
      ];
    default:
      return null;
  }
};

const getTags = (tagsOption: TagsOption): Record<string, string> | null => {
  switch (tagsOption) {
    case 'none':
      return null;
    case 'many':
      return {
        Environment: 'production',
        Team: 'platform',
        'teleport.dev/example': 'testing',
      };
    default:
      return null;
  }
};

const generateTerraformModule = (props: StoryProps): string => {
  const clusterName = props.cluster_name || null;
  const regions = getRegions(props.region_count);
  const tags = getTags(props.tags);
  const complexType = [
    { testing: ['1', 2, false, { Test: 'example' }] },
    { enabled: true },
  ];

  const multiline = `hello
  world`;
  return hcl`# Example Terraform Module

module "example" {
  source = "../"
  
  cluster_name  = ${clusterName}

  # array type
  regions = ${regions}

  # map type
  tags = ${tags}

  # multiline string
  multiline = ${multiline}

  # boolean
  enabled = ${props.is_enabled}

  # complex type
  complex_type = ${complexType}
}`;
};

export function TerraformModule(props: StoryProps) {
  const terraformContent = generateTerraformModule(props);

  return (
    <Flex height="600px" width="800px" py={3} pr={3} bg="levels.deep">
      <LiveTextEditor
        bg="levels.deep"
        data={[
          {
            content: terraformContent,
            type: 'terraform',
          },
        ]}
      />
    </Flex>
  );
}
