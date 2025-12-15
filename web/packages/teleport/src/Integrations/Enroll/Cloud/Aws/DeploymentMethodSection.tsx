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

import { Box, ButtonSecondary, Flex, H2, Text } from 'design';
import { FieldRadio } from 'design/FieldRadio';
import { Copy } from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import { useValidation } from 'shared/components/Validation';

type DeploymentMethod = 'terraform' | 'manual';

type DeploymentMethodSectionProps = {
  deploymentMethod: DeploymentMethod;
  onChange: (method: DeploymentMethod) => void;
  terraformConfig?: string;
};

export function DeploymentMethodSection({
  deploymentMethod,
  onChange,
  terraformConfig,
}: DeploymentMethodSectionProps) {
  const validator = useValidation();

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
          <Flex flexDirection="column" mb={3} gap={2}>
            <Text bold={true} fontSize={1}>
              1. Add the Teleport module to your Terraform configuration
            </Text>
            <Text>
              Copy the module on the right and paste it into your Terraform
              configuration.
            </Text>
            <Box>
              <ButtonSecondary
                disabled={!terraformConfig}
                onClick={() => {
                  if (validator.validate() && terraformConfig) {
                    copyToClipboard(terraformConfig);
                  }
                }}
                gap={2}
              >
                <Copy size="small" />
                Copy Configuration
              </ButtonSecondary>
              {!validator.state.valid && (
                <Text color="error.main" mt={2} fontSize={1}>
                  Please complete the required fields
                </Text>
              )}
            </Box>
            <Text bold={true} fontSize={1}>
              2. Initialize and apply the configuration
            </Text>
            <Text>
              After the command completes successfully, the integration will be
              registered automatically and auto-discovery will begin.
            </Text>
            <TextSelectCopyMulti lines={[{ text: `terraform apply` }]} />
            <Text bold={true} fontSize={1}>
              3. Return to Teleport to verify the integration
            </Text>
          </Flex>
        )}
      </Box>
    </>
  );
}

const RadioLabel = styled(Flex)<{ selected: boolean }>`
  font-weight: ${props => (props.selected ? '600' : 'inherit')};
`;
