import { MemoryRouter } from 'react-router';

import { render, screen, waitFor } from 'design/utils/testing';

import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { allAccessAcl } from 'teleport/mocks/contexts';
import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { ExternalAuditStorageProvider } from '../useExternalAuditStorage';
import { SelectIntegration } from './SelectIntegration';

describe('selectIntegration', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  const setup = () => {
    const ctx = new TeleportEContext();
    ctx.storeUser.setState({
      username: 'joe@example.com',
      acl: allAccessAcl,
    });

    jest.spyOn(integrationService, 'fetchIntegrations').mockResolvedValue({
      items: [
        {
          resourceType: 'integration',
          name: 'my-custom-aws-integration',
          kind: IntegrationKind.AwsOidc,
          statusCode: IntegrationStatusCode.Running,
          spec: { roleArn: '' },
        },
      ],
    });

    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <ExternalAuditStorageProvider>
            <SelectIntegration />
          </ExternalAuditStorageProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  it('renders AWS integrations select', async () => {
    await waitFor(() => setup());
    expect(
      screen.getByText(/Select the name of the AWS integration to use:/)
    ).toBeInTheDocument();
  });

  it('disables the Next button when no integration is selected', async () => {
    await waitFor(() => setup());
    expect(screen.getByText(/Next/)).toBeDisabled();
  });
});
