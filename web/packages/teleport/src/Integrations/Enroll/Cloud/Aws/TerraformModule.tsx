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

import LiveTextEditor from '../LiveTextEditor';
import { hcl } from '../terraform';
import { AwsConfig, AwsLabel, Ec2Config } from './types';

type TerraformModuleProps = {
  awsConfig: AwsConfig;
  ec2Config: Ec2Config;
  onContentChange?: (content: string) => void;
};

export function TerraformModule(props: TerraformModuleProps) {
  const content = makeTerraform(props);
  props.onContentChange?.(content);

  return (
    <Flex height="600px" width="100%" m={0}>
      <LiveTextEditor
        data={[{ content, type: 'terraform' }]}
        bg="levels.deep"
      />
    </Flex>
  );
}

const makeTerraform = ({ awsConfig, ec2Config }: TerraformModuleProps) => {
  const integration = awsConfig.integration.name.trim() || '<integration_name>';
  const isWildcard =
    awsConfig.regions.length === 1 && awsConfig.regions[0] === '*';
  const regions = isWildcard ? null : awsConfig.regions.sort();

  const filterMatchers = (tags: AwsLabel[]) => {
    const filtered = tags.filter(o => o.value || o.name);
    return filtered.length > 0 ? filtered : null;
  };

  const ec2Enabled = ec2Config.enabled || null;

  const ec2Matchers = ec2Enabled ? filterMatchers(ec2Config.tags) : null;

  const tfModule = hcl`# Terraform Module
module ${`teleport_aws_discovery_${integration}`} {
  source = "github.com/gravitational/teleport.git/examples/modules/whatever"

  integration_name = ${integration}

  regions = ${regions}

  ec2_enabled = ${ec2Enabled}

  ec2_matchers = ${ec2Matchers}
}`;

  return tfModule;
};
