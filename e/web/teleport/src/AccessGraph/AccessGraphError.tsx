import { useEffect } from 'react';
import type { FallbackProps } from 'react-error-boundary';
import styled from 'styled-components';

import { Alert, Box, ButtonLink, Flex } from 'design';
import { Logger } from 'design/logger';

import { storageService } from 'teleport/services/storageService';

const logger = new Logger('AccessGraph/AccessGraphError');

const Container = styled.div`
  display: flex;
  flex: 1;
  width: 100%;
  flex-direction: column;
  position: relative;
  justify-content: center;
  align-items: center;

  > div {
    max-width: 500px;
  }
`;

interface AccessGraphSetupErrorProps {
  isConfigured: boolean;
}

export function AccessGraphSetupError({
  isConfigured,
}: AccessGraphSetupErrorProps) {
  if (isConfigured) {
    return (
      <Container>
        <Alert
          kind="danger"
          alignItems="flex-start"
          details={
            <>
              <Box mt={2}>
                If you recently enabled Identity Security, it will become
                available after your next maintenance window.
              </Box>
              <Box mt={2}>
                Otherwise, please ensure the Access Graph service is running and
                that your Teleport cluster is able to communicate with it.
              </Box>
              <DocumentationButton />
            </>
          }
        >
          Identity Security is available and configured for your Teleport
          cluster, but the Access Graph service cannot be contacted.
        </Alert>
      </Container>
    );
  }

  return (
    <Container>
      <Alert
        kind="danger"
        alignItems="flex-start"
        details={<DocumentationButton />}
      >
        Identity Security is available for your Teleport cluster, but it has not
        been configured yet.
      </Alert>
    </Container>
  );
}

// Fallback component for the error boundaries around the lazily-loaded Identity
// Security bundles. A truthy `getAccessGraphEnabled` means `/features.json` was
// served by Access Graph, so the service is reachable and the failure is most
// likely a version incompatibility between it and Teleport.
export function AccessGraphLoadingError({ error }: FallbackProps) {
  // Logged rather than displayed: a failed script load only rejects with an
  // opaque DOM load event, which makes for unhelpful copy.
  useEffect(() => {
    logger.error('failed to load Identity Security', error);
  }, [error]);

  if (storageService.getAccessGraphEnabled()) {
    return (
      <Container>
        <Alert
          kind="danger"
          alignItems="flex-start"
          details={
            <>
              <Box mt={2}>
                Identity Security failed to load, which usually means the
                version running in your cluster is incompatible with this
                version of Teleport. Please ensure it has been updated.
              </Box>
              <DocumentationButton />
            </>
          }
        >
          Identity Security may be incompatible with this version of Teleport.
        </Alert>
      </Container>
    );
  }

  return (
    <Container>
      <Alert
        kind="danger"
        alignItems="flex-start"
        details={<DocumentationButton />}
      >
        Could not load Identity Security. Please ensure the Access Graph service
        is running.
      </Alert>
    </Container>
  );
}

function DocumentationButton() {
  return (
    <Flex justifyContent="flex-end" mt={2}>
      <ButtonLink
        href="https://goteleport.com/docs/identity-security/access-graph/"
        target="_blank"
      >
        View Documentation
      </ButtonLink>
    </Flex>
  );
}
