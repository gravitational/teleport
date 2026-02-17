/* eslint-disable testing-library/no-node-access */
import { act } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';
import selectEvent from 'react-select-event';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { SessionSummariesManagementProvider } from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';

const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => {
  server.resetHandlers();
  return testQueryClient.resetQueries();
});
afterAll(() => server.close());

test('shows Bedrock configuration form when Bedrock is selected', async () => {
  mockListSecrets();
  mockListIntegrations([]);

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);

  expect(screen.getByText('Bedrock Model Configuration')).toBeInTheDocument();
});

test('shows three tabs in Bedrock configuration', async () => {
  mockListSecrets();
  mockListIntegrations([]);

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);

  expect(screen.getByText('Use Teleport AWS integration')).toBeInTheDocument();
  expect(screen.getByText('Connect directly to a model')).toBeInTheDocument();
  expect(screen.getByText('Use an inference profile')).toBeInTheDocument();
});

test('integration selector becomes enabled after selecting region and model', async () => {
  mockListSecrets();
  mockListIntegrations([
    {
      name: 'my-aws-integration',
      subKind: 'aws-oidc',
      awsoidc: { roleArn: 'arn:aws:iam::123456789012:role/TeleportRole' },
    },
  ]);

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);

  const integrationLabel = await screen.findByText(
    'Teleport AWS Integration to use'
  );

  const selectContainer = integrationLabel
    .closest('label')
    ?.querySelector('.react-select__control');
  expect(selectContainer).toHaveClass('react-select__control--is-disabled');

  await selectRegion('us-east-1');

  await enterModelName(user, 'anthropic.claude-3-sonnet');

  await waitFor(() => {
    const selectContainer = screen
      .getByText('Teleport AWS Integration to use')
      .closest('label')
      ?.querySelector('.react-select__control');

    expect(selectContainer).not.toHaveClass(
      'react-select__control--is-disabled'
    );
  });
});

test('shows integrations in dropdown after filling region and model', async () => {
  mockListSecrets();
  mockListIntegrations([
    {
      name: 'my-aws-integration',
      subKind: 'aws-oidc',
      awsoidc: { roleArn: 'arn:aws:iam::123456789012:role/TeleportRole' },
    },
    {
      name: 'another-integration',
      subKind: 'aws-oidc',
      awsoidc: { roleArn: 'arn:aws:iam::987654321098:role/AnotherRole' },
    },
  ]);
  mockTestInferenceModel({ success: true });

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);
  await selectRegion('us-east-1');
  await enterModelName(user, 'anthropic.claude-3-sonnet');

  await waitFor(() => {
    const selectContainer = screen
      .getByText('Teleport AWS Integration to use')
      .closest('label')
      ?.querySelector('.react-select__control');
    expect(selectContainer).not.toHaveClass(
      'react-select__control--is-disabled'
    );
  });

  const integrationSelect = screen.getByText('Select...');
  await selectEvent.openMenu(integrationSelect);

  expect(await screen.findByText('my-aws-integration')).toBeInTheDocument();
  expect(await screen.findByText('another-integration')).toBeInTheDocument();
});

test('selecting integration triggers connection test', async () => {
  mockListSecrets();
  mockListIntegrations([
    {
      name: 'test-integration',
      subKind: 'aws-oidc',
      awsoidc: { roleArn: 'arn:aws:iam::123456789012:role/TestRole' },
    },
  ]);
  mockTestInferenceModel({ success: true });

  renderCreateInferenceModel();

  const user = userEvent.setup();

  await selectBedrockProvider(user);
  await selectRegion('us-east-1');
  await enterModelName(user, 'anthropic.claude-3-sonnet');

  await waitFor(() => {
    const selectContainer = screen
      .getByText('Teleport AWS Integration to use')
      .closest('label')
      ?.querySelector('.react-select__control');

    expect(selectContainer).not.toHaveClass(
      'react-select__control--is-disabled'
    );
  });

  const integrationSelect = screen.getByText('Select...');
  await act(async () => {
    await selectEvent.select(integrationSelect, 'test-integration');
  });

  await waitFor(() => {
    expect(
      screen.getByText('Permissions are correctly configured')
    ).toBeInTheDocument();
  });
});

test('shows CloudShell instructions when permissions are missing', async () => {
  mockListSecrets();
  mockListIntegrations([
    {
      name: 'needs-permissions',
      subKind: 'aws-oidc',
      awsoidc: { roleArn: 'arn:aws:iam::123456789012:role/NeedsPermissions' },
    },
  ]);
  mockTestInferenceModel({
    success: false,
    message: 'Access denied to Bedrock',
  });

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);
  await selectRegion('us-east-1');
  await enterModelName(user, 'anthropic.claude-3-sonnet');

  await waitFor(() => {
    const selectContainer = screen
      .getByText('Teleport AWS Integration to use')
      .closest('label')
      ?.querySelector('.react-select__control');
    expect(selectContainer).not.toHaveClass(
      'react-select__control--is-disabled'
    );
  });

  const integrationSelect = screen.getByText('Select...');
  await act(async () => {
    await selectEvent.select(integrationSelect, 'needs-permissions');
  });

  await waitFor(() => {
    expect(screen.getByText('CloudShell Instructions')).toBeInTheDocument();
  });

  expect(screen.getByText(/Access denied to Bedrock/i)).toBeInTheDocument();
  expect(screen.getByRole('link', { name: /AWS CloudShell/i })).toHaveAttribute(
    'href',
    'https://console.aws.amazon.com/cloudshell/home'
  );
});

test('Direct tab does not show integration selector', async () => {
  mockListSecrets();
  mockListIntegrations([]);

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);

  await user.click(screen.getByText('Connect directly to a model'));

  expect(
    screen.queryByText('Teleport AWS Integration to use')
  ).not.toBeInTheDocument();
  expect(
    screen.getByText(/To connect directly to a Bedrock model/i)
  ).toBeInTheDocument();
});

test('Inference Profile tab shows ARN input instead of model select', async () => {
  mockListSecrets();
  mockListIntegrations([]);

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);

  await user.click(screen.getByText('Use an inference profile'));

  expect(screen.getByLabelText(/Inference Profile ARN/i)).toBeInTheDocument();
  expect(
    screen.queryByText('Teleport AWS Integration to use')
  ).not.toBeInTheDocument();
});

test('inference profile ARN auto-populates region', async () => {
  mockListSecrets();
  mockListIntegrations([]);

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);

  await user.click(screen.getByText('Use an inference profile'));

  const arnInput = screen.getByLabelText(/Inference Profile ARN/i);
  await user.type(
    arnInput,
    'arn:aws:bedrock:us-west-2:123456789012:inference-profile/my-profile'
  );

  await waitFor(() => {
    expect(screen.getByText('us-west-2')).toBeInTheDocument();
  });
});

test('isCloud hides non-integration tabs', async () => {
  cfg.oss.isCloud = true;

  mockListSecrets();
  mockListIntegrations([]);

  renderCreateInferenceModel();

  const user = userEvent.setup();
  await selectBedrockProvider(user);

  expect(screen.getByText('Bedrock Model Configuration')).toBeInTheDocument();
  expect(
    screen.queryByText('Use Teleport AWS integration')
  ).not.toBeInTheDocument();
  expect(
    screen.queryByText('Connect directly to a model')
  ).not.toBeInTheDocument();
  expect(
    screen.queryByText('Use an inference profile')
  ).not.toBeInTheDocument();

  expect(
    screen.getByText('Teleport AWS Integration to use')
  ).toBeInTheDocument();

  cfg.oss.isCloud = false;
});

interface MockIntegration {
  name: string;
  subKind: string;
  awsoidc: {
    roleArn: string;
  };
}

function mockListIntegrations(integrations: MockIntegration[] = []) {
  server.use(
    http.get(cfg.oss.api.integrationsPath, () =>
      HttpResponse.json({ items: integrations })
    )
  );
}

function mockListSecrets() {
  server.use(
    http.get(cfg.api.inference.secrets, () => HttpResponse.json({ items: [] }))
  );
}

function mockTestInferenceModel(response: {
  success: boolean;
  message?: string;
}) {
  server.use(
    http.post(cfg.api.inference.testModel, () => HttpResponse.json(response))
  );
}

function renderCreateInferenceModel() {
  return render(
    <MemoryRouter initialEntries={['/#new-model']}>
      <SessionSummariesManagementProvider />
    </MemoryRouter>
  );
}

async function selectBedrockProvider(user: ReturnType<typeof userEvent.setup>) {
  const bedrockOption = screen.getByText('Amazon Bedrock');
  await user.click(bedrockOption);
}

async function selectRegion(regionLabel: string) {
  const regionSelect = screen.getByText('Model Region').closest('label');
  const regionInput = regionSelect?.querySelector('input');

  if (regionInput) {
    await selectEvent.select(regionInput, regionLabel);
  }
}

async function enterModelName(
  user: ReturnType<typeof userEvent.setup>,
  modelName: string
) {
  const modelLabel = screen.getByText('Model', { exact: true });
  const modelInput = modelLabel.closest('label')?.querySelector('input');

  if (modelInput) {
    await user.type(modelInput, modelName);
  }
}
