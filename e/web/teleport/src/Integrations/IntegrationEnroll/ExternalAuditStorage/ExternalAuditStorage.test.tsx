import { MemoryRouter } from 'react-router';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { externalAuditStorageService } from 'e-teleport/services/externalauditstorage';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { externalAuditStorage } from 'teleport/Integrations/fixtures';
import { allAccessAcl } from 'teleport/mocks/contexts';
import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { ExternalAuditStorage } from './ExternalAuditStorage';
import { ExternalAuditStorageProvider } from './useExternalAuditStorage';

describe('externalAuditStorage', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  const setup = () => {
    const ctx = new TeleportEContext();
    ctx.storeUser.setState({
      username: 'joe@example.com',
      acl: allAccessAcl,
    });

    jest
      .spyOn(ctx.externalAuditStorageService, 'getDraft')
      .mockResolvedValue(null);

    jest
      .spyOn(ctx.externalAuditStorageService, 'generateDraft')
      .mockResolvedValue(externalAuditStorage);

    jest
      .spyOn(ctx.externalAuditStorageService, 'promoteDraft')
      .mockResolvedValue(null);

    jest
      .spyOn(ctx.externalAuditStorageService, 'testConnection')
      .mockResolvedValue({
        id: 'id',
        success: true,
        message: 'test ok',
        traces: [],
      });

    jest.spyOn(integrationService, 'fetchIntegrations').mockResolvedValue({
      items: [
        {
          resourceType: 'integration',
          name: 'my-custom-aws-integration',
          kind: IntegrationKind.AwsOidc,
          statusCode: IntegrationStatusCode.Running,
          spec: { roleArn: '', issuerS3Bucket: '', issuerS3Prefix: '' },
        },
      ],
    });

    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <ExternalAuditStorageProvider>
            <ExternalAuditStorage />
          </ExternalAuditStorageProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  test('renders with initial state', async () => {
    setup();

    await waitFor(() => {
      // Assertions for elements that should be present in the initial state
      expect(
        screen.getByText('Configure External Audit Storage')
      ).toBeInTheDocument();
    });
    expect(
      screen.getByText(/Teleport will create S3 buckets/)
    ).toBeInTheDocument();
    expect(screen.getByText('Step 1: Select Integration')).toBeInTheDocument();

    // steps 2 and 3 should not be shown yet
    expect(
      screen.queryByText('Step 2: Configure Permissions')
    ).not.toBeInTheDocument();

    expect(
      screen.queryByText('Step 3: Test Connection')
    ).not.toBeInTheDocument();
  });

  test('renders success message and buttons when activated', async () => {
    setup();
    // click through the flow to activate it

    // step 1
    expect(screen.getByText('Step 1: Select Integration')).toBeInTheDocument();
    const awsDropdown = await screen.findByText(
      'Select the AWS Integration to Use'
    );
    await userEvent.click(awsDropdown);
    const item = screen.getByText('my-custom-aws-integration');
    expect(item).toBeInTheDocument();
    await userEvent.click(item);
    await userEvent.click(screen.getByText('Next'));

    // step 2
    expect(
      screen.getByText('Step 2: Configure Permissions')
    ).toBeInTheDocument();
    await userEvent.click(screen.getByText('Generate Script'));
    expect(screen.getByText(/bash -c/)).toBeInTheDocument();

    // step 3
    expect(screen.getByText('Step 3: Test Connection')).toBeInTheDocument();
    await userEvent.click(screen.getByTestId('test-button'));

    // activate
    await userEvent.click(screen.getByTestId('activate-button'));

    expect(
      screen.getByText('External Audit Storage Successfully Added')
    ).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Go to Integration List' })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Add Another Integration' })
    ).toBeInTheDocument();

    expect(screen.queryByText(/Draft in progress/)).not.toBeInTheDocument();
  });

  it('renders warning if a previous draft exists', async () => {
    setup();
    jest
      .spyOn(externalAuditStorageService, 'getDraft')
      .mockResolvedValue(externalAuditStorage);

    // go through step 1
    const awsDropdown = await screen.findByText(
      'Select the AWS Integration to Use'
    );
    await userEvent.click(awsDropdown);
    const item = screen.getByText('my-custom-aws-integration');
    await userEvent.click(item);
    await userEvent.click(screen.getByText('Next'));

    // generate script
    await userEvent.click(screen.getByText('Generate Script'));
    expect(screen.getByText(/Draft in progress/)).toBeInTheDocument();
    expect(screen.getByText(/Continue draft/)).toBeInTheDocument();
    expect(screen.getByText(/Delete draft/)).toBeInTheDocument();
    expect(screen.getByText(/Cancel/)).toBeInTheDocument();
  });

  it('continues the previous draft', async () => {
    setup();
    jest
      .spyOn(externalAuditStorageService, 'getDraft')
      .mockResolvedValue(externalAuditStorage);

    // go through step 1
    const awsDropdown = await screen.findByText(
      'Select the AWS Integration to Use'
    );
    await userEvent.click(awsDropdown);
    const item = screen.getByText('my-custom-aws-integration');
    await userEvent.click(item);
    await userEvent.click(screen.getByText('Next'));

    // generate script
    await userEvent.click(screen.getByText('Generate Script'));
    await userEvent.click(screen.getByText(/Continue draft/));
    expect(screen.getByTestId(/test-button/)).toBeInTheDocument();
  });
});
