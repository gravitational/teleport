import styled from 'styled-components';

import { Alert, Box, ButtonLink, Flex } from 'design';

import { storageService } from 'teleport/services/storageService';

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

export function AccessGraphLoadingError() {
  if (storageService.getAccessGraphEnabled()) {
    // If we're here, it means we failed to load the React 19 specific JS bundle from Access Graph,
    // but have successfully loaded `/features.json` from Access Graph (which indicates that the
    // service is running and the feature is enabled).
    // This likely means that the version of Access Graph is old and incompatible with Teleport 18+
    // (where React 19 is used).
    // TODO(ryan): delete in v19 as v17 will no longer be supported and this error should never be seen again.
    return (
      <Container>
        <Alert
          kind="danger"
          alignItems="flex-start"
          details={<DocumentationButton />}
        >
          The current version of Identity Security is incompatible with Teleport
          18. Please update to the latest version.
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
