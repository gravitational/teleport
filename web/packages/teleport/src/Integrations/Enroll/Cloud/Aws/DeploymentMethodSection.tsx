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

import { useState } from 'react';
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';

import { Alert, Box, Button, ButtonText, Flex, Text } from 'design';
import { Check, Copy, Notification, Spinner } from 'design/Icon';
import { rotate360 } from 'design/keyframes';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import { useValidation } from 'shared/components/Validation';

import cfg from 'teleport/config';

import { CircleNumber } from './EnrollAws';

export function CopyTerraformButton({
  onClick,
}: {
  onClick: (e: React.SyntheticEvent) => void;
}) {
  const [configCopied, setConfigCopied] = useState(false);

  const handleClick = (e: React.SyntheticEvent) => {
    onClick(e);

    if (!e.defaultPrevented) {
      setConfigCopied(true);
      setTimeout(() => setConfigCopied(false), 1000);
    }
  };

  return (
    <Button fill="border" intent="primary" onClick={handleClick} gap={2}>
      {configCopied ? <Check size="small" /> : <Copy size="small" />}
      Copy Terraform Module
    </Button>
  );
}

type DeploymentMethodSectionProps = {
  terraformConfig?: string;
  handleCopy: () => void;
  integrationExists?: boolean;
  integrationName?: string;
  onCheckIntegration?: () => void;
  onCancelCheckIntegration?: () => void;
  isCheckingIntegration?: boolean;
  showVerificationStep?: boolean;
};

export function DeploymentMethodSection({
  handleCopy,
  integrationExists,
  integrationName,
  onCheckIntegration,
  isCheckingIntegration,
  onCancelCheckIntegration,
  showVerificationStep = true,
}: DeploymentMethodSectionProps) {
  const validator = useValidation();

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
        <CircleNumber>5</CircleNumber>
        Deployment Method
      </Flex>
      <Box ml={4} mb={3}>
        <Text mb={3}>
          Deploy the required IAM resources in your AWS account using Terraform.
        </Text>
        <Text fontSize={3} fontWeight="regular">
          Terraform
        </Text>
        <Text>
          Automatically provision IAM roles and policies using Infrastructure as
          Code.
          <br />
          Best for: Teams managing infrastructure with Terraform.
        </Text>
      </Box>

      <Box ml={6}>
        <Flex flexDirection="column" mb={3} gap={2}>
          <Text bold={true} fontSize="14px">
            1. Add the Teleport AWS Discovery module to your Terraform
            configuration
          </Text>
          <Text>
            Copy the module on the right and paste it into your Terraform
            configuration.
          </Text>
          <Box>
            <CopyTerraformButton
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
          <Text bold={true} fontSize="14px">
            2. Initialize and apply the configuration
          </Text>
          <Text>
            Run the following commands in your terminal. <br />
            Initialize Terraform to download the module, then apply the
            configuration to create the integration and configure the discovery
            service.
          </Text>
          <TextSelectCopyMulti
            lines={[{ text: `terraform init` }, { text: `terraform apply` }]}
          />
          {showVerificationStep && (
            <Box>
              <Text bold={true} fontSize="14px" mb={2}>
                3. Verify the integration
              </Text>
              {integrationExists ? (
                <Alert kind="success" mb={2}>
                  Integration Detected
                  <Text fontWeight="regular">
                    Amazon Web Services successfully added
                  </Text>
                </Alert>
              ) : (
                <>
                  <Box mb={3}>
                    {isCheckingIntegration ? (
                      <Button
                        fill="filled"
                        intent="primary"
                        disabled={true}
                        onClick={onCheckIntegration}
                        gap={2}
                      >
                        <AnimatedSpinner size="small" />
                        Checking...
                      </Button>
                    ) : (
                      <Button
                        fill="filled"
                        intent="primary"
                        disabled={false}
                        onClick={onCheckIntegration}
                        gap={2}
                      >
                        Check Integration
                      </Button>
                    )}
                  </Box>
                  <Box mb={3}>
                    {isCheckingIntegration ? (
                      <Alert
                        kind="info"
                        icon={Notification}
                        primaryAction={{
                          content: 'Cancel',
                          onClick: onCancelCheckIntegration,
                        }}
                        mb={0}
                      >
                        Checking for integration '{integrationName}'...
                      </Alert>
                    ) : (
                      <Alert kind="neutral" icon={Notification} mb={0}>
                        <Text fontWeight="regular" color="text.slightlyMuted">
                          After applying your Terraform configuration, verify
                          your integration was created successfully.
                        </Text>
                      </Alert>
                    )}
                  </Box>
                  <Box
                    pl={3}
                    borderLeft="2px solid"
                    borderColor="interactive.tonal.neutral.0"
                  >
                    <Flex gap={2} flexDirection="column">
                      <Text bold={true} fontSize={1}>
                        Don't want to wait?
                      </Text>
                      <Text>
                        Once you've successfully applied your Terraform
                        configuration, the integration will be available on the
                        Integrations page.
                      </Text>
                      <Box css={{ position: 'relative', left: '-8px' }}>
                        <InternalLink to={cfg.routes.integrations}>
                          <ButtonText intent="primary" size="small">
                            View Integrations
                          </ButtonText>
                        </InternalLink>
                      </Box>
                    </Flex>
                  </Box>
                </>
              )}
            </Box>
          )}
        </Flex>
      </Box>
    </>
  );
}

const AnimatedSpinner = styled(Spinner)`
  animation: ${rotate360} 1.5s linear infinite;
`;
