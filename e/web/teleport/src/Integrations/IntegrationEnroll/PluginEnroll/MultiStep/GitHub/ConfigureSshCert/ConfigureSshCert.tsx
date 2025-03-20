import { useEffect } from 'react';

import {
  Box,
  ButtonPrimary,
  Link as ExternalLink,
  Flex,
  Indicator,
  Mark,
  Text,
} from 'design';
import { Alert, Info } from 'design/Alert/Alert';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import { useAsync } from 'shared/hooks/useAsync';
import { getErrMessage } from 'shared/utils/errorType';

import { StyledBox } from 'teleport/Discover/Shared';
import { integrationService } from 'teleport/services/integrations';
import { IntegrationEnrollStatusCode } from 'teleport/services/userEvent';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { Header } from '../../Shared';
import { getIntegrationName } from '../getIntegrationName';
import { Props } from '../GitHub';

export function ConfigureSshCert({
  gitHubOrgName,
  nextStep,
  emitEvent,
}: Props) {
  const { clusterId } = useStickyClusterId();

  const [fetchCaAttempt, fetchCa] = useAsync(async () => {
    try {
      return await integrationService.fetchExportedIntegrationCA(
        clusterId,
        getIntegrationName(gitHubOrgName)
      );
    } catch (err) {
      emitEvent({
        status: {
          code: IntegrationEnrollStatusCode.Error,
          error: `Failed to fetch CA: ${getErrMessage(err)}`,
        },
      });
      throw err;
    }
  });

  function onNext() {
    emitEvent({ status: { code: IntegrationEnrollStatusCode.Success } });
    nextStep();
  }

  useEffect(() => {
    fetchCa();
  }, []);

  let publicKey: string, fingerprint: string;
  if (fetchCaAttempt.status === 'success' && fetchCaAttempt.data.ssh?.length) {
    publicKey = fetchCaAttempt.data.ssh[0].publicKey;
    fingerprint = fetchCaAttempt.data.ssh[0].fingerprint;
  }

  return (
    <>
      <Box mb={3} pt={3}>
        <Header header="Configure SSH certificate authority on GitHub" />
        <Text>
          Add Teleport generated SSH certificate authority to your GitHub
          organization.
        </Text>
      </Box>
      {fetchCaAttempt.status === 'error' && (
        <Alert
          kind="danger"
          primaryAction={{ content: 'Retry', onClick: fetchCa }}
        >
          {fetchCaAttempt.statusText}
        </Alert>
      )}
      {(fetchCaAttempt.status === 'processing' ||
        fetchCaAttempt.status === '') && (
        <Box textAlign="center" m={10} width="350px">
          <Indicator />
        </Box>
      )}
      {fetchCaAttempt.status === 'success' && (
        <>
          <StyledBox mb={5}>
            <Box mb={3}>
              <Text>
                Click on <Mark>New CA</Mark> in your organization's
                <Mark>Authentication security</Mark> settings under{' '}
                <ExternalLink
                  href={`https://github.com/organizations/${gitHubOrgName}/settings/ssh_certificate_authorities/new`}
                  target="_blank"
                >
                  SSH certificate authorities
                </ExternalLink>{' '}
                section. Copy and paste Teleport's CA onto the <Mark>Key</Mark>{' '}
                input box:
              </Text>
              <TextSelectCopyMulti bash={false} lines={[{ text: publicKey }]} />
            </Box>
            <Text>
              After clicking on <Mark>Add CA</Mark>, double check that its{' '}
              <Mark>SHA256 Fingerprint</Mark> (under the{' '}
              <Mark>SSH certificate authorities</Mark> section) is same as
              below:
            </Text>
            <TextSelectCopyMulti bash={false} lines={[{ text: fingerprint }]} />
          </StyledBox>
          <Info width="800px">
            Under <Mark>SSH certificate authorities</Mark> there is an option to{' '}
            <ExternalLink
              href="https://docs.github.com/en/enterprise-cloud@latest/admin/enforcing-policies/enforcing-policies-for-your-enterprise/enforcing-policies-for-security-settings-in-your-enterprise#managing-ssh-certificate-authorities-for-your-enterprise"
              target="_blank"
            >
              Require SSH Certificates
            </ExternalLink>
            . It is recommended to enable this option once all GitHub users has
            migrated to Teleport. Once enabled, it will force users to use
            Teleport for connectivity.
          </Info>
          <Flex mt={4} mb={5} gap={3}>
            <ButtonPrimary
              onClick={onNext}
              disabled={fetchCaAttempt.status !== 'success'}
            >
              Next
            </ButtonPrimary>
          </Flex>
        </>
      )}
    </>
  );
}
