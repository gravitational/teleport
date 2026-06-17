/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Alert, Box, Flex, Link as ExternalLink, Text } from 'design';
import { ArrowSquareOut, Notification, Spinner } from 'design/Icon';
import { rotate360 } from 'design/keyframes';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import { useValidation } from 'shared/components/Validation';

import { TerraformCopyButton } from 'teleport/components/TerraformCopyButton';
import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';

import { CircleNumber, Divider } from '../Shared';

type ApplyTerraformSectionProps = {
  handleCopy: () => void;
  integrationExists?: boolean;
  integrationName?: string;
  handleCheckIntegration?: () => void;
  handleCancelCheckIntegration?: () => void;
  checkIntegrationError?: boolean;
  isCheckingIntegration?: boolean;
  showVerificationStep?: boolean;
};

export function ApplyTerraformSection({
  handleCopy,
  integrationExists,
  integrationName,
  handleCheckIntegration,
  isCheckingIntegration,
  checkIntegrationError,
  handleCancelCheckIntegration,
  showVerificationStep = true,
}: ApplyTerraformSectionProps) {
  const validator = useValidation();

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={2}>
        <CircleNumber>4</CircleNumber>
        Apply Terraform
      </Flex>

      <Box ml={4}>
        <Flex flexDirection="column" mb={3}>
          <Box mt={1} mb={4}>
            <Text bold={true} fontSize="14px">
              1. Add the module generated on the right to your Terraform
              templates.
            </Text>
            <Box mt={2}>
              <TerraformCopyButton
                onClick={e => {
                  const isValid = validator.validate();
                  if (!isValid) {
                    e.preventDefault();
                  } else {
                    handleCopy();
                  }
                }}
              />
              {validator.state.validating && !validator.state.valid && (
                <Text color="error.main" mt={2} fontSize={1}>
                  Please complete the required fields
                </Text>
              )}
            </Box>
          </Box>
          <Box mb={4}>
            <Text bold={true} fontSize="14px">
              2. Configure Azure and Teleport providers
            </Text>
            <Text color="text.slightlyMuted" fontSize={1}>
              If you need help configuring providers for your environment,
              please reference <br />
              <ExternalLink
                href="https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/"
                target="_blank"
              >
                <Flex inline alignItems="center">
                  Teleport Terraform provider
                  <ArrowSquareOut size={12} ml={1} />
                </Flex>
              </ExternalLink>{' '}
              <ExternalLink
                href="https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs"
                target="_blank"
              >
                <Flex inline alignItems="center">
                  Azure Terraform provider
                  <ArrowSquareOut size={12} ml={1} />
                </Flex>
              </ExternalLink>
            </Text>
            <Text mt={1}>
              Generate temporary bot Teleport credentials for Terraform.
            </Text>
            <TextSelectCopyMulti
              lines={[
                {
                  comment: `tsh login --proxy=${cfg.proxyCluster}`,
                  text: `eval "$(tctl terraform env)"`,
                },
              ]}
            />
          </Box>
          <Box>
            <Text bold={true} fontSize="14px" mb={2}>
              3. Initialize and apply the configuration
            </Text>
            <TextSelectCopyMulti
              lines={[{ text: `terraform init` }, { text: `terraform apply` }]}
            />
          </Box>
        </Flex>
      </Box>

      {showVerificationStep && (
        <>
          <Divider />
          <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={2}>
            <CircleNumber>5</CircleNumber>
            Verify the Integration
          </Flex>

          <Box ml={4} mt={1}>
            {integrationExists ? (
              <Alert
                kind="success"
                mb={2}
                primaryAction={{
                  content: 'View Integration',
                  linkTo: cfg.getIaCIntegrationRoute(
                    IntegrationKind.AzureOidc,
                    integrationName
                  ),
                }}
              >
                Integration Detected
                <Text fontWeight="regular">Azure successfully added</Text>
              </Alert>
            ) : (
              <Box mb={3}>
                {isCheckingIntegration ? (
                  <Alert
                    kind="info"
                    icon={AnimatedSpinner}
                    primaryAction={{
                      content: 'Cancel',
                      onClick: handleCancelCheckIntegration,
                    }}
                    mb={0}
                  >
                    <Text fontWeight="regular" color="text.slightlyMuted">
                      Checking for integration{' '}
                      <Text as="span" fontWeight="bold">
                        {integrationName}
                      </Text>
                      ...
                    </Text>
                  </Alert>
                ) : checkIntegrationError ? (
                  <Alert
                    kind="danger"
                    mb={0}
                    primaryAction={{
                      content: 'Check Integration',
                      onClick: handleCheckIntegration,
                    }}
                  >
                    Failed to detect integration
                    <Text fontWeight="regular" color="text.slightlyMuted">
                      Unable to detect the Azure integration "{integrationName}
                      ". Please check your Terraform configuration and try
                      again.
                    </Text>
                  </Alert>
                ) : (
                  <Alert
                    kind="neutral"
                    icon={Notification}
                    mb={0}
                    primaryAction={{
                      content: 'Check Integration',
                      onClick: handleCheckIntegration,
                    }}
                  >
                    <Text fontWeight="regular" color="text.slightlyMuted">
                      After applying your Terraform configuration, verify your
                      integration was created successfully.
                    </Text>
                  </Alert>
                )}
              </Box>
            )}
          </Box>
        </>
      )}
    </>
  );
}

const AnimatedSpinner = styled(Spinner)`
  animation: ${rotate360} 1.5s linear infinite;
`;
