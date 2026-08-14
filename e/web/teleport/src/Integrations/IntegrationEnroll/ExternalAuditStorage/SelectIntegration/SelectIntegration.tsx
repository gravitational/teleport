import { useEffect, useState } from 'react';
import { Link } from 'react-router';

import { Alert, Box, ButtonPrimary, ButtonText, Text } from 'design';
import Select, { Option as BaseOption } from 'shared/components/Select';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import {
  IntegrationAwsOidc,
  IntegrationKind,
  integrationService,
  IntegrationUrlLocationState,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { Step, useExternalAuditStorage } from '../useExternalAuditStorage';

type Option = BaseOption<IntegrationAwsOidc>;

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

  useEffect(() => {
    if (!hasAccess) {
      return;
    }
    run(() =>
      integrationService.fetchIntegrations().then(res => {
        const options = (res.items || [])
          .filter(i => i.kind === 'aws-oidc')
          .map(i => ({ value: i, label: i.name })) satisfies Option[];
        setAwsIntegrations(options);
      })
    );
  }, [hasAccess, run]);

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
      {!hasAccess ? (
        <Alert mt="4">
          <Text>
            Insufficient permissions. Reach out to your Teleport administrator
            to request permissions to list and read integrations.
          </Text>
        </Alert>
      ) : hasAwsIntegrations ? (
        <>
          <Text mb={2}>Select the name of the AWS integration to use:</Text>
          <Box width="300px" mb={2}>
            <Select
              isDisabled={currentStep !== Step.SelectIntegration}
              placeholder="Select the AWS Integration to Use"
              isSearchable
              value={selectedOption}
              onChange={v => setSelectedAwsIntegration(v?.value ?? null)}
              options={awsIntegrations}
              data-testid="aws-integration-select"
            />
          </Box>
          <ButtonText
            as={Link}
            to={integrationEnrollPath}
            state={locationState}
            compact
          >
            Or click here to set up a different AWS account
          </ButtonText>

          <Box mt="2">
            <ButtonPrimary
              onClick={nextStep}
              disabled={
                currentStep !== Step.SelectIntegration ||
                !selectedAwsIntegration
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
            to={integrationEnrollPath}
            state={locationState}
            disabled={!hasAccess}
          >
            Set up AWS Account
          </ButtonPrimary>
        </Box>
      )}
    </>
  );
}
const integrationEnrollPath = cfg.getIntegrationEnrollRoute(
  IntegrationKind.AwsOidc
);
const locationState: { integration: IntegrationUrlLocationState } = {
  integration: {
    kind: IntegrationKind.ExternalAuditStorage,
    redirectText: 'Continue External Audit Storage Setup',
  },
};
