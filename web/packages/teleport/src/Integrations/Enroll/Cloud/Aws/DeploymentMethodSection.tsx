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

import styled from 'styled-components';

import { Box, Flex, H2, Text } from 'design';
import { FieldRadio } from 'design/FieldRadio';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import { IntegrationConfig } from './types';

type DeploymentMethod = 'terraform' | 'manual';

type DeploymentMethodSectionProps = {
  deploymentMethod: DeploymentMethod;
  onChange: (method: DeploymentMethod) => void;
  integration: IntegrationConfig;
  onRoleChange: (roleArn: string) => void;
};

export function DeploymentMethodSection({
  deploymentMethod,
  onChange,
}: DeploymentMethodSectionProps) {
  return (
    <>
      <H2>Deployment Method</H2>
      <Text mb={3}>
        Choose how you would like to deploy AWS IAM and Teleport resources used
        to discover resources.
      </Text>
      <Box>
        <FieldRadio
          name="deployment-method"
          label={
            <Flex alignItems="center" gap={2}>
              <RadioLabel selected={deploymentMethod === 'terraform'}>
                Terraform (Recommended)
              </RadioLabel>
            </Flex>
          }
          helperText="Automatically provision IAM roles and policies using Infrastructure
  as Code."
          value="terraform"
          checked={deploymentMethod === 'terraform'}
          onChange={() => onChange('terraform')}
          mb={3}
        />

        {deploymentMethod === 'terraform' && (
          <Flex flexDirection="column" ml={5} mb={3} gap={2}>
            <Text>Add the Teleport module to your Terraform configuration</Text>
            <Text>
              Copy the module on the right and paste it into your Terraform
              configuration.
            </Text>
            <TextSelectCopyMulti lines={[{ text: `terraform apply` }]} />
          </Flex>
        )}
        <FieldRadio
          name="deployment-method"
          label={
            <Flex alignItems="center" gap={2}>
              <RadioLabel selected={deploymentMethod === 'manual'}>
                Manual
              </RadioLabel>
            </Flex>
          }
          helperText="Manually create and configure the required IAM roles and policies."
          value="manual"
          checked={deploymentMethod === 'manual'}
          onChange={() => onChange('manual')}
          mb={3}
        />

        {deploymentMethod === 'manual' && (
          <Flex flexDirection="column" ml={5} mb={2} gap={2}>
            <Text>Manual deployment goes here...</Text>
          </Flex>
        )}
      </Box>
    </>
  );
}

const RadioLabel = styled(Flex)<{ selected: boolean }>`
  font-weight: ${props => (props.selected ? '600' : 'inherit')};
`;
