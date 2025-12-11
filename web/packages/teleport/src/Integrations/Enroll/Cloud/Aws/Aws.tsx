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

import { useRef, useState } from 'react';
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';

import { Box, ButtonPrimary, Flex, H2, Subtitle1, Text } from 'design';
import { Spinner } from 'design/Icon';
import { rotate360 } from 'design/keyframes';
import FieldInput from 'shared/components/FieldInput';
import { useToastNotifications } from 'shared/components/ToastNotification';
import Validation from 'shared/components/Validation';
import { requiredIntegrationName } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';

import { DeploymentMethodSection } from './DeploymentMethodSection';
import { RegionsSection } from './RegionsSection';
import { ResourcesSection } from './ResourcesSection';
import { TerraformModule } from './TerraformModule';
import { useAws } from './useAws';

type deploymentMethod = 'terraform' | 'manual';

const POLLING_INTERVAL_MS = 5000;
const POLLING_TIMEOUT_MS = 30000;

export function Aws() {
  const { awsConfig, setAwsConfig, setIntegration, setEc2Config } = useAws();
  const [deploymentMethod, setDeploymentMethod] =
    useState<deploymentMethod>('terraform');

  const [isPollingIntegration, setIsPollingIntegration] = useState(false);
  const [integrationExists, setIntegrationExists] = useState(false);
  const [terraformConfig, setTerraformConfig] = useState('');
  const toastNotifications = useToastNotifications();
  const abortControllerRef = useRef<AbortController>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval>>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout>>(null);

  const integration = awsConfig.integration;
  const ec2Config = awsConfig.ec2Config;

  const onIntegrationSuccess = () => {
    toastNotifications.add({
      severity: 'success',
      content: {
        title: 'Amazon Web Services successfully added',
        description:
          'Amazon Web Services has been successfully added ' +
          'to this Teleport Cluster. Your resources will appear ' +
          "automatically as they're discovered. This may take a few minutes.",
        action: {
          content: 'View Integration',
          linkTo: cfg.getIntegrationStatusRoute(
            IntegrationKind.AwsOidc,
            integration.name
          ),
        },
        isAutoRemovable: false,
      },
    });
  };

  const onIntegrationFailure = () => {
    toastNotifications.add({
      severity: 'error',
      content: {
        title: 'Failed to detect integration',
        description:
          'Unable to detect the AWS integration. Please check your configuration and try again.',
        action: {
          content: 'Check Integration',
          onClick: pollIntegration,
        },
      },
    });
  };

  const stopPolling = () => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = undefined;
    }
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = undefined;
    }
    setIsPollingIntegration(false);
  };

  const checkIntegrationExists = async () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    abortControllerRef.current = new AbortController();

    try {
      const foundIntegration = await integrationService.fetchIntegration(
        integration.name,
        abortControllerRef.current?.signal
      );
      if (foundIntegration) {
        stopPolling();
        setIntegrationExists(true);
        onIntegrationSuccess();
      }
    } catch (error) {
      if (error.name === 'AbortError') {
        return;
      }
      if (error?.response?.status !== 404) {
        stopPolling();
        onIntegrationFailure();
      }
    }
  };

  const pollIntegration = () => {
    if (!integration.name || isPollingIntegration) return;
    setIsPollingIntegration(true);

    checkIntegrationExists();

    intervalRef.current = setInterval(
      checkIntegrationExists,
      POLLING_INTERVAL_MS
    );

    timeoutRef.current = setTimeout(() => {
      stopPolling();
      onIntegrationFailure();
    }, POLLING_TIMEOUT_MS);
  };

  return (
    <Validation>
      {({ validator }) => (
        <Flex pt={3}>
          <Box flex="1" mr={3}>
            <Header>Connect Amazon Web Services</Header>
            <Subtitle1 mb={3}>
              Connect your AWS account to automatically discover and enroll
              resources in your Teleport Cluster.
            </Subtitle1>
            <Container flexDirection="column" p={6}>
              <H2>Integration Details</H2>
              <Text>
                A unique name to identify this AWS integration. This will be
                used to reference the integration in Teleport.
              </Text>
              <FieldInput
                autoFocus={true}
                rule={requiredIntegrationName}
                value={integration.name}
                required={true}
                label="Integration name"
                placeholder="Integration Name"
                maxWidth={360}
                mt={2}
                onChange={e =>
                  setIntegration(prev => ({
                    ...prev,
                    name: e.target.value.trim(),
                  }))
                }
              />
              <ResourcesSection
                ec2Config={ec2Config}
                onEc2Change={config => setEc2Config(() => config)}
              />
              <RegionsSection
                regions={awsConfig.regions}
                onChange={regions => setAwsConfig({ ...awsConfig, regions })}
              />
              <DeploymentMethodSection
                deploymentMethod={deploymentMethod}
                onChange={setDeploymentMethod}
                terraformConfig={terraformConfig}
              />
            </Container>
            <Box mt={3}>
              <IntegrationButton
                integrationName={integration.name}
                integrationKind={IntegrationKind.AwsOidc}
                integrationExists={integrationExists}
                isPolling={isPollingIntegration}
                onClick={() => {
                  if (validator.validate()) {
                    pollIntegration();
                  }
                }}
              />
              <Text mt={2} color="text.secondary">
                or{' '}
                <Text as="a" href={cfg.routes.integrations} color="text.main">
                  view all integrations
                </Text>
              </Text>
            </Box>
          </Box>
          <Box
            width="420px"
            style={{ position: 'sticky', top: 20, alignSelf: 'flex-start' }}
          >
            <TerraformModule
              awsConfig={awsConfig}
              ec2Config={ec2Config}
              onContentChange={setTerraformConfig}
            />
          </Box>
        </Flex>
      )}
    </Validation>
  );
}

const Container = styled(Flex)`
  border-radius: 8px;
  background: ${props => props.theme.colors.levels.elevated};

  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

type IntegrationButtonProps = {
  isPolling: boolean;
  integrationName: string;
  onClick: () => void;
  integrationExists: boolean;
  integrationKind?: IntegrationKind;
};

const IntegrationButton = ({
  isPolling,
  integrationName,
  onClick,
  integrationExists,
  integrationKind = IntegrationKind.AwsOidc,
}: IntegrationButtonProps) => {
  if (integrationExists) {
    return (
      <ButtonPrimary
        as={InternalLink}
        to={cfg.getIntegrationStatusRoute(integrationKind, integrationName)}
      >
        View Integration
      </ButtonPrimary>
    );
  }

  return (
    <ButtonPrimary disabled={isPolling} onClick={onClick} gap={2}>
      {isPolling && <AnimatedSpinner size="small" />}
      {isPolling ? 'Checking...' : 'Check Integration'}
    </ButtonPrimary>
  );
};

const AnimatedSpinner = styled(Spinner)`
  animation: ${rotate360} 1.5s linear infinite;
`;
