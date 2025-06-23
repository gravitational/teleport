import styled from 'styled-components';

import { Alert } from 'design';

import { storageService } from 'teleport/services/storageService';

const Container = styled.div`
  display: flex;
  flex: 1;
  width: 100%;
  flex-direction: column;
  position: relative;
  justify-content: center;
  align-items: center;
`;

export function AccessGraphError() {
  if (storageService.getAccessGraphEnabled()) {
    return (
      <Container>
        <Alert kind="danger">
          The current version of Identity Security is incompatible with Teleport
          18. Please update to the latest version.
        </Alert>
      </Container>
    );
  }

  return (
    <Container>
      <Alert kind="danger">
        Could not load Identity Security. Please ensure the Identity Security
        service is running.
      </Alert>
    </Container>
  );
}
