/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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

import cfg from 'teleport/config';
import {
  Integration,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import { integrationRWEAndDbCU } from 'teleport/Discover/yamlTemplates';
import useTeleport from 'teleport/useTeleport';

import { ActionButtons, HeaderSubtitle, Header } from '../../Shared';

import {
  DbMeta,
  DiscoverUrlLocationState,
  useDiscover,
} from '../../useDiscover';

type Option = BaseOption<Integration>;

export function ConnectAwsAccount() {
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
  const databaseAccess = storeUser.getDatabaseAccess();
  const hasAccess =
    integrationAccess.create &&
    integrationAccess.list &&
    // Required access after integrating:
    integrationAccess.use && // required to list AWS RDS db's
    databaseAccess.create; // required to enroll AWS RDS db
  const { attempt, run } = useAttempt(hasAccess ? 'processing' : '');

  const [awsIntegrations, setAwsIntegrations] = useState<Option[]>([]);
  const [selectedAwsIntegration, setSelectedAwsIntegration] =
    useState<Option>();

  useEffect(() => {
    if (hasAccess) {
      fetchAwsIntegrations();
    }
  }, []);

  function fetchAwsIntegrations() {
    run(() =>
      integrationService.fetchIntegrations().then(res => {
        const options = res.items.map(i => {
          if (i.kind === 'aws-oidc') {
            return {
              value: i,
              label: i.name,
            };
          }
        });
        setAwsIntegrations(options);
      })
    );
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
              data={[{ content: integrationRWEAndDbCU, type: 'yaml' }]}
            />
          </Flex>
        </Box>
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
        <ButtonPrimary mt={2} onClick={fetchAwsIntegrations}>
          Retry
        </ButtonPrimary>
      </Box>
    );
  }

  function proceedWithExistingIntegration(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    updateAgentMeta({
      ...(agentMeta as DbMeta),
      integration: selectedAwsIntegration.value,
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
