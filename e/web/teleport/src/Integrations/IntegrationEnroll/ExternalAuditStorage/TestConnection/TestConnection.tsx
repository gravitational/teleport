import { useState } from 'react';

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleportE from 'e-teleport/useTeleportE';
import { Header, HeaderSubtitle, TextIcon } from 'teleport/Discover/Shared';
import { ErrorWithDetails } from 'teleport/Discover/Shared/ConnectionDiagnostic';
import { ConnectionDiagnostic } from 'teleport/services/agents';

import { useExternalAuditStorage } from '../useExternalAuditStorage';

export function TestConnection() {
  const { nextStep } = useExternalAuditStorage();
  const { attempt, run } = useAttempt('');
  const [testResult, setTestResult] = useState<ConnectionDiagnostic>(null);
  const { externalAuditStorageService } = useTeleportE();

  function handleTest() {
    run(() =>
      externalAuditStorageService.testConnection().then(result => {
        setTestResult(result);
        if (result.success) {
          nextStep();
        }
      })
    );
    nextStep();
  }

  return (
    <>
      <Header>Test Connection</Header>
      <HeaderSubtitle>
        Once the script has finished running in AWS CloudShell, click the
        &quot;Test Connection&quot; button to check that your integration is set
        up correctly to store audit logs and session recordings.
      </HeaderSubtitle>
      <Flex>
        <ButtonSecondary
          mr="4"
          disabled={attempt.status == 'processing'}
          onClick={handleTest}
          data-testid="test-button"
        >
          {testResult ? 'Restart Test' : 'Test Connection'}
        </ButtonSecondary>
        {attempt.status == 'processing' && (
          <TextIcon>
            <Icons.Restore size="medium" mr={2} />
            Testing in-progress
          </TextIcon>
        )}
        {attempt.status == 'failed' && (
          <TextIcon>
            <Icons.Warning size="medium" color="error.main" />
            Testing failed: {attempt.statusText}
          </TextIcon>
        )}
        {attempt.status == 'success' && (
          <TextIcon>
            <Icons.CircleCheck size="medium" color="success.main" />
            Testing complete
          </TextIcon>
        )}
      </Flex>
      {attempt.status === 'success' && (
        <Box mt="3">
          {testResult?.traces.map((trace, index) => {
            if (trace.status === 'failed') {
              return (
                <ErrorWithDetails
                  error={trace.error}
                  details={trace.details}
                  key={index}
                />
              );
            }
            if (trace.status === 'success') {
              return (
                <TextIcon key={index}>
                  <Icons.CircleCheck
                    size="medium"
                    color="success.main"
                    mr={1}
                  />
                  {trace.details}
                </TextIcon>
              );
            }

            // For whatever reason the status is not the value
            // of failed or success.
            return (
              <TextIcon key={index}>
                <Icons.Question size="medium" mr={1} />
                {trace.details}
              </TextIcon>
            );
          })}
        </Box>
      )}
    </>
  );
}
