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

import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Box,
  ButtonText,
  Text,
  ButtonPrimary,
  Indicator,
  Alert,
  Flex,
} from 'design';
import FieldSelect from 'shared/components/FieldSelect';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Option as BaseOption } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import TextEditor from 'shared/components/TextEditor';
import { getErrMessage } from 'shared/utils/errorType';

import { App } from 'teleport/services/apps';
import cfg from 'teleport/config';
import {
  Integration,
  IntegrationKind,
  IntegrationListResponse,
  integrationService,
} from 'teleport/services/integrations';
import {
  integrationRWE,
  integrationRWEAndNodeRWE,
  integrationRWEAndDbCU,
  integrationAndAppRW,
} from 'teleport/Discover/yamlTemplates';
import useTeleport from 'teleport/useTeleport';
import ResourceService from 'teleport/services/resources';

import {
  ActionButtons,
  HeaderSubtitle,
  Header,
  ResourceKind,
} from '../../Shared';

import { DiscoverUrlLocationState, useDiscover } from '../../useDiscover';

type Option = BaseOption<Integration>;

export function AwsAccount() {
  const { storeUser } = useTeleport();
  const {
    prevStep,
    nextStep,
    agentMeta,
    updateAgentMeta,
    eventState,
    resourceSpec,
    currentStep,
  } = useDiscover();

  const integrationAccess = storeUser.getIntegrationsAccess();

  // if true, requires an additional step where we fetch for
  // apps matching fetched aws integrations to determine
  // if an app already exists for the integration the user
  // will select
  const isAddingAwsApp =
    resourceSpec.kind === ResourceKind.Application &&
    resourceSpec.appMeta.awsConsole;

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

  const { attempt, setAttempt } = useAttempt(hasAccess ? 'processing' : '');

  // only used if resource we are trying to add is an AWS console.
  const [fetchedApps, setFetchedApps] = useState<App[]>([]);

  const [awsIntegrations, setAwsIntegrations] = useState<Option[]>([]);
  const [selectedAwsIntegration, setSelectedAwsIntegration] =
    useState<Option>();

  useEffect(() => {
    if (hasAccess) {
      fetchAwsIntegrationsWithApps();
    }
  }, []);

  function makeAwsIntegrationOptions(res: IntegrationListResponse) {
    const options = res.items
      .filter(i => i.kind === 'aws-oidc')
      .map(i => ({
        value: i,
        label: i.name,
      }));
    setAwsIntegrations(options);

    // Auto select the only option.
    if (options.length === 1) {
      setSelectedAwsIntegration(options[0]);
    }

    return options;
  }

  async function fetchAwsIntegrationsWithApps() {
    setAttempt({ status: 'processing' });
    try {
      let integrationOptions = awsIntegrations;
      // checking length, if re-attempting from failure
      if (integrationOptions.length === 0) {
        const integrationPage = await integrationService.fetchIntegrations();
        integrationOptions = makeAwsIntegrationOptions(integrationPage);
      }
      if (!isAddingAwsApp || integrationOptions.length === 0) {
        setAttempt({ status: 'success' });
        // Skip fetching for apps
        return;
      }

      // checking length, if re-attempting from failure
      if (fetchedApps.length === 0) {
        const resourceSvc = new ResourceService();
        // fetch for apps that match fetched integration names.
        // used later to determine if the integration user selected
        // already has a application created for it.
        const query = integrationOptions
          .map(i => i.value.name)
          .map(
            integrationName =>
              `resource.spec.integration == "${integrationName}"`
          )
          .join(' || ');

        const { agents: resources } = await resourceSvc.fetchUnifiedResources(
          storeUser.getClusterId(),
          {
            query,
            limit: integrationOptions.length,
            kinds: ['app'],
            sort: { fieldName: 'name', dir: 'ASC' },
          }
        );
        setFetchedApps(resources.filter(r => r.kind === 'app'));
        setAttempt({ status: 'success' });
      }
    } catch (err) {
      const errMsg = getErrMessage(err);
      setAttempt({ status: 'failed', statusText: errMsg });
    }
  }

  if (!hasAccess) {
    return (
      <Box maxWidth="700px">
        <Heading />
        <Box maxWidth="700px">
          <Text mt={4} width="100px">
            You don’t have the required permissions for integrating.
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

  if (attempt.status === 'processing') {
    return (
      <Box maxWidth="700px">
        <Heading />
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </Box>
    );
  }

  if (attempt.status === 'failed') {
    return (
      <Box maxWidth="700px">
        <Heading />
        <Alert kind="danger" children={attempt.statusText} />
        <ButtonPrimary mt={2} onClick={fetchAwsIntegrationsWithApps}>
          Retry
        </ButtonPrimary>
      </Box>
    );
  }

  function proceedWithExistingIntegration(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    if (isAddingAwsApp && fetchedApps.length > 0) {
      // See if an application already exists for selected integration
      const foundApp = fetchedApps.find(
        // app => app.name === selectedAwsIntegration.value.name
        app =>
          app.integration === selectedAwsIntegration.value.name &&
          app.awsConsole
      );
      if (foundApp) {
        updateAgentMeta({
          ...agentMeta,
          awsIntegration: selectedAwsIntegration.value,
          app: foundApp,
          awsRoleArns: foundApp.awsRoles.map(r => r.arn),
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
                      disabled
                      label="AWS Integrations"
                      rule={requiredField('Region is required')}
                      placeholder="Select the AWS Integration to Use"
                      isSearchable
                      isSimpleValue
                      value={selectedAwsIntegration}
                      onChange={i => setSelectedAwsIntegration(i as Option)}
                      options={awsIntegrations}
                    />
                  </Box>
                  <ButtonText as={Link} to={locationState} pl={0}>
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
                disableProceed={!hasAwsIntegrations || !selectedAwsIntegration}
              />
            </>
          )}
        </Validation>
      </Box>
    </Box>
  );
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
