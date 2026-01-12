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

import { Link as InternalLink } from 'react-router-dom';

import { Alert, Box, ButtonSecondary, Flex, Text } from 'design';
import { ArrowSquareOut, Copy, Notification } from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import { useValidation } from 'shared/components/Validation';

import cfg from 'teleport/config';

import { CircleNumber } from './EnrollAws';

type DeploymentMethodSectionProps = {
  terraformConfig?: string;
  copyConfigButtonRef?: React.RefObject<HTMLButtonElement>;
  integrationExists?: boolean;
};

export function DeploymentMethodSection({
  terraformConfig,
  copyConfigButtonRef,
  integrationExists,
}: DeploymentMethodSectionProps) {
  const validator = useValidation();

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium">
        <CircleNumber>5</CircleNumber>
        Deployment Method
      </Flex>
      <Box ml={4} mb={3}>
        <Text mb={2}>
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
            1. Add the Teleport module to your Terraform configuration
          </Text>
          <Text>
            Copy the module on the right and paste it into your Terraform
            configuration.
          </Text>
          <Box>
            <ButtonSecondary
              ref={copyConfigButtonRef}
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
          <Text bold={true} fontSize="14px">
            2. Initialize and apply the configuration
          </Text>
          <Text>
            Run the following command in your terminal. After applying, Teleport
            will verify the integration in the background.
          </Text>
          <TextSelectCopyMulti lines={[{ text: `terraform apply` }]} />
          <Text bold={true} fontSize="14px">
            3. Verify the Integration
          </Text>
          {integrationExists ? (
            <Alert kind="success" mb={0}>
              <Text fontWeight="regular">
                Amazon Web Services successfully added
              </Text>
            </Alert>
          ) : (
            <>
              <Alert kind="neutral" icon={Notification} mb={0}>
                <Text fontWeight="regular" color="text.slightlyMuted">
                  After applying your Terraform configuration, we'll
                  automatically detect your new integration on this page.
                </Text>
              </Alert>
              <Box
                pl={4}
                borderLeft="2px solid"
                borderColor="interactive.tonal.neutral.0"
              >
                <Text bold={true} fontSize={1}>
                  Don't want to wait?
                </Text>
                <Text mb={2}>
                  You'll receive an in-app notification when ready.
                </Text>
                <Text>
                  If the detection is taking longer than expected, view the
                  integrations list after the Terraform configuration has
                  successfully applied.
                </Text>
                <Box>
                  <InternalLink to={cfg.routes.integrations}>
                    <Flex alignItems="center" gap={1}>
                      View Integrations
                      <ArrowSquareOut size="small" />
                    </Flex>
                  </InternalLink>
                </Box>
              </Box>
            </>
          )}
        </Flex>
      </Box>
    </>
  );
}
