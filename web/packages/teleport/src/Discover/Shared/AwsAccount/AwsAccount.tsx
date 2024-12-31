/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonText,
  Flex,
  Indicator,
  Text,
} from 'design';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option as BaseOption } from 'shared/components/Select';
import TextEditor from 'shared/components/TextEditor';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'teleport/config';
import {
  integrationAndAppRW,
  integrationRWE,
  integrationRWEAndDbCU,
  integrationRWEAndNodeRWE,
} from 'teleport/Discover/yamlTemplates';
import { App } from 'teleport/services/apps';
import {
  Integration,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import ResourceService from 'teleport/services/resources';
import useTeleport from 'teleport/useTeleport';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  ResourceKind,
} from '../../Shared';
import { DiscoverUrlLocationState, useDiscover } from '../../useDiscover';

type Option = BaseOption<Integration>;

export function AwsAccount() {
  const {
    prevStep,
    nextStep,
    agentMeta,
    updateAgentMeta,
    eventState,
    resourceSpec,
    currentStep,
    emitErrorEvent,
  } = useDiscover();

  const [selectedAwsIntegration, setSelectedAwsIntegration] =
    useState<Option>();

  // if true, requires an additional step where we fetch for
  // apps matching fetched aws integrations to determine
  // if an app already exists for the integration the user
  // will select
  const isAddingAwsApp =
    resourceSpec.kind === ResourceKind.Application &&
    resourceSpec.appMeta.awsConsole;

  const { storeUser } = useTeleport();
  const clusterId = storeUser.getClusterId();

  const [attempt, fetch] = useAsync(
    useCallback(async () => {
      const response = await fetchAwsIntegrationsWithApps(
        clusterId,
        isAddingAwsApp
      );
      // Auto select the only option.
      if (response.awsIntegrations.length === 1) {
        setSelectedAwsIntegration(
          makeAwsIntegrationOption(response.awsIntegrations[0])
        );
      }
      return response;
    }, [clusterId, isAddingAwsApp])
  );

  const [healthCheckAttempt, healthCheckSelectedIntegration] = useAsync(
    async () => {
      await integrationService.pingAwsOidcIntegration(
        {
          clusterId,
          integrationName: selectedAwsIntegration.value.name,
        },
        { roleArn: '' }
      );
    }
  );

  const integrationAccess = storeUser.getIntegrationsAccess();

  let roleTemplate = integrationRWE;
  let hasAccess =
    integrationAccess.create &&
    integrationAccess.list &&
    integrationAccess.use &&
    integrationAccess.read;

  // Ensure required permissions based on which flow this is in.
  if (resourceSpec.kind === ResourceKind.Database) {
    roleTemplate = integrationRWEAndDbCU;
    const databaseAccess = storeUser.getDatabaseAccess();
    hasAccess = hasAccess && databaseAccess.create; // required to enroll AWS RDS db
  }
  if (resourceSpec.kind === ResourceKind.Server) {
    roleTemplate = integrationRWEAndNodeRWE;
    const nodesAccess = storeUser.getNodeAccess();
    hasAccess =
      hasAccess &&
      nodesAccess.create &&
      nodesAccess.edit &&
      nodesAccess.list &&
      nodesAccess.read; // Needed for TestConnection flow
  }
  if (isAddingAwsApp) {
    roleTemplate = integrationAndAppRW;
    const appAccess = storeUser.getAppServerAccess();
    hasAccess =
      hasAccess &&
      // required to upsert app server
      appAccess.create &&
      appAccess.edit &&
      // required to list and read app servers
      appAccess.list &&
      appAccess.read;
  }

  useEffect(() => {
    if (hasAccess && attempt.status === '') {
      fetch();
    }
  }, [attempt.status, fetch, hasAccess]);

  if (!hasAccess) {
    return (
      <Box maxWidth="700px">
        <Heading />
        <Box maxWidth="700px">
          <Text mt={4}>
            You don’t have the permissions required to set up this integration.
            <br />
            Ask your Teleport administrator to update your role with the
            following:
          </Text>
          <Flex minHeight="215px" mt={3}>
            <TextEditor
              readOnly={true}
              bg="levels.deep"
              data={[{ content: roleTemplate, type: 'yaml' }]}
            />
          </Flex>
        </Box>
        <ActionButtons onPrev={prevStep} />
      </Box>
    );
  }

  if (attempt.status === '' || attempt.status === 'processing') {
    return (
      <Box maxWidth="700px">
        <Heading />
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </Box>
    );
  }

  if (attempt.status === 'error') {
    return (
      <Box maxWidth="700px">
        <Heading />
        <Alert kind="danger" children={attempt.statusText} />
        <ButtonPrimary mt={2} onClick={fetch}>
          Retry
        </ButtonPrimary>
      </Box>
    );
  }

  async function proceedWithExistingIntegration(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    const [, err] = await healthCheckSelectedIntegration();
    if (err) {
      emitErrorEvent(`failed to health check selected aws integration: ${err}`);
      return;
    }

    if (
      isAddingAwsApp &&
      attempt.status === 'success' &&
      attempt.data.apps.length > 0
    ) {
      // See if an application already exists for selected integration
      const foundApp = attempt.data.apps.find(
        app =>
          app.integration === selectedAwsIntegration.value.name &&
          app.awsConsole
      );
      if (foundApp) {
        updateAgentMeta({
          ...agentMeta,
          awsIntegration: selectedAwsIntegration.value,
          app: foundApp,
        });
        // skips the next step (creating an app server)
        // since it already exists
        nextStep(2);
        return;
      }
    }

    updateAgentMeta({
      ...agentMeta,
      awsIntegration: selectedAwsIntegration.value,
    });

    nextStep();
  }

  const { awsIntegrations } = attempt.data;
  const hasAwsIntegrations = awsIntegrations.length > 0;

  // When a user clicks to create a new AWS integration, we
  // define location state to preserve all the states required
  // to resume from this step when the user comes back to discover route
  // after successfully finishing enrolling integration.
  const locationState = {
    pathname: cfg.getIntegrationEnrollRoute(IntegrationKind.AwsOidc),
    state: {
      discover: {
        eventState,
        resourceSpec,
        currentStep,
      },
    } as DiscoverUrlLocationState,
  };
  return (
    <Box maxWidth="700px">
      <Heading />
      {healthCheckAttempt.status === 'error' && (
        <Alert
          kind="danger"
          children={`Health check failed for the selected AWS integration: ${healthCheckAttempt.statusText}`}
        />
      )}
      <Box mb={3}>
        <Validation>
          {({ validator }) => (
            <>
              {hasAwsIntegrations ? (
                <>
                  <Text mb={2}>
                    Select the name of the AWS integration to use:
                  </Text>
                  <Box width="300px" mb={6}>
                    <FieldSelect
                      label="AWS Integrations"
                      rule={requiredField<Option>('Region is required')}
                      placeholder="Select the AWS Integration to Use"
                      isSearchable
                      value={selectedAwsIntegration}
                      onChange={i => setSelectedAwsIntegration(i as Option)}
                      options={awsIntegrations.map(makeAwsIntegrationOption)}
                    />
                  </Box>
                  <ButtonText as={Link} to={locationState} compact>
                    Or click here to set up a different AWS account
                  </ButtonText>
                </>
              ) : (
                <ButtonPrimary
                  mt={2}
                  mb={2}
                  size="large"
                  as={Link}
                  to={locationState}
                >
                  Set up AWS Account
                </ButtonPrimary>
              )}

              <ActionButtons
                onPrev={prevStep}
                onProceed={() => proceedWithExistingIntegration(validator)}
                disableProceed={
                  !hasAwsIntegrations ||
                  !selectedAwsIntegration ||
                  healthCheckAttempt.status === 'processing'
                }
              />
            </>
          )}
        </Validation>
      </Box>
    </Box>
  );
}

function makeAwsIntegrationOption(integration: Integration): Option {
  return {
    value: integration,
    label: integration.name,
  };
}

async function fetchAwsIntegrationsWithApps(
  clusterId: string,
  isAddingAwsApp: boolean
): Promise<{
  awsIntegrations: Integration[];
  apps: App[];
}> {
  const integrationPage = await integrationService.fetchIntegrations();
  const awsIntegrations = integrationPage.items.filter(
    i => i.kind === 'aws-oidc'
  );
  if (!isAddingAwsApp || awsIntegrations.length === 0) {
    // Skip fetching for apps
    return { awsIntegrations, apps: [] };
  }

  const resourceSvc = new ResourceService();
  // fetch for apps that match fetched integration names.
  // used later to determine if the integration user selected
  // already has an application created for it.
  const query = awsIntegrations
    .map(i => `resource.spec.integration == "${i.name}"`)
    .join(' || ');

  const { agents: resources } = await resourceSvc.fetchUnifiedResources(
    clusterId,
    {
      query,
      limit: awsIntegrations.length,
      kinds: ['app'],
      sort: { fieldName: 'name', dir: 'ASC' },
    }
  );
  return { awsIntegrations, apps: resources as App[] };
}

const Heading = () => (
  <>
    <Header>Connect to your AWS Account</Header>
    <HeaderSubtitle>
      Instead of storing long-lived static credentials, Teleport will request
      short-lived credentials from AWS to perform operations automatically.
    </HeaderSubtitle>
  </>
);
