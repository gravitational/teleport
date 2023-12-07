import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Text, Box, ButtonText, ButtonPrimary, Alert } from 'design';
import cfg from 'teleport/config';
import Select, { Option as BaseOption } from 'shared/components/Select';
import {
  Integration,
  IntegrationKind,
  IntegrationUrlLocationState,
  integrationService,
} from 'teleport/services/integrations';
import useAttempt from 'shared/hooks/useAttemptNext';
import useTeleport from 'teleport/useTeleport';

import { Step, useExternalAuditStorage } from '../useExternalAuditStorage';
type Option = BaseOption<Integration>;

export function SelectIntegration() {
  const { storeUser } = useTeleport();
  const {
    currentStep,
    nextStep,
    selectedAwsIntegration,
    setSelectedAwsIntegration,
  } = useExternalAuditStorage();

  const integrationAccess = storeUser.getIntegrationsAccess();
  const hasAccess = integrationAccess.list && integrationAccess.read;

  const [awsIntegrations, setAwsIntegrations] = useState<Option[]>([]);
  const { attempt, run } = useAttempt('');

  const selectedOption: Option = selectedAwsIntegration
    ? {
        label: selectedAwsIntegration.name,
        value: selectedAwsIntegration,
      }
    : null;

  function setSelected(o: Option) {
    setSelectedAwsIntegration(o.value);
  }

  function handleConfirm() {
    nextStep();
  }

  useEffect(() => {
    if (hasAccess) {
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
  }, []);

  if (attempt.status === 'processing') {
    return <Text>Loading...</Text>;
  }
  if (attempt.status === 'failed') {
    return (
      <Alert>Error loading current integrations: {attempt.statusText}</Alert>
    );
  }
  const hasAwsIntegrations = awsIntegrations.length > 0;
  return (
    <>
      {!hasAccess && (
        <Alert mt="4">
          <Text>
            Insuficient permissions. Reach out to your Teleport administrator to
            request permissions to list and read{' '}
            <Text bold style={{ display: 'inline' }}>
              integrations
            </Text>
            .
          </Text>
        </Alert>
      )}
      {hasAwsIntegrations ? (
        <>
          <Text mb={2}>Select the name of the AWS integration to use:</Text>
          <Box width="300px" mb={2}>
            <Select
              isDisabled={currentStep !== Step.SelectIntegration}
              label="AWS Integrations"
              placeholder="Select the AWS Integration to Use"
              isSearchable
              isSimpleValue
              value={selectedOption}
              onChange={setSelected}
              options={awsIntegrations}
              data-testid="aws-integration-select"
            />
          </Box>
          <ButtonText as={Link} to={locationState} pl={0}>
            Or click here to set up a different AWS account
          </ButtonText>

          <Box mt="2">
            <ButtonPrimary
              onClick={handleConfirm}
              disabled={
                currentStep !== Step.SelectIntegration ||
                selectedAwsIntegration === null
              }
            >
              Next
            </ButtonPrimary>
          </Box>
        </>
      ) : (
        <Box>
          <Text>
            In order to continue, you have to setup an AWS OIDC integration
            first.
          </Text>
          <ButtonPrimary
            mt={2}
            mb={2}
            as={Link}
            to={locationState}
            disabled={!hasAccess}
          >
            Set up AWS Account
          </ButtonPrimary>
        </Box>
      )}
    </>
  );
}
const locationState: {
  pathname: string;
  state: { integration: IntegrationUrlLocationState };
} = {
  pathname: cfg.getIntegrationEnrollRoute(IntegrationKind.AwsOidc),
  state: {
    integration: {
      kind: IntegrationKind.ExternalAuditStorage,
      redirectText: 'Continue External Audit Storage Setup',
    },
  },
};
