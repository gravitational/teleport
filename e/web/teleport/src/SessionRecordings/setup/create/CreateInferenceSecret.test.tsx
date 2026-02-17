import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import type { InferenceSecret } from 'e-teleport/services/inference/types';
import { SessionSummariesManagementProvider } from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';

enableMswServer();

afterEach(() => {
  testQueryClient.clear();
});

test('renders the create dialog with correct title', () => {
  renderCreateInferenceSecret();

  expect(screen.getByText('Create Inference Secret')).toBeInTheDocument();
});

test('renders the form fields', () => {
  renderCreateInferenceSecret();

  expect(screen.getByLabelText(/API Key/i)).toBeInTheDocument();
  expect(screen.getByLabelText(/Name/i)).toBeInTheDocument();
});

test('create button is disabled when form is invalid', () => {
  renderCreateInferenceSecret();

  const createButton = screen.getByRole('button', { name: /Create/i });

  expect(createButton).toBeDisabled();
});

test('create button is enabled when form is valid', async () => {
  renderCreateInferenceSecret();

  const user = userEvent.setup();

  const apiKeyInput = screen.getByLabelText(/API Key/i);
  const nameInput = screen.getByLabelText(/Name/i);

  await user.type(apiKeyInput, 'sk-test-api-key');
  await user.type(nameInput, 'my-secret');

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Create/i })).toBeEnabled();
  });
});

test('successfully creates an inference secret', async () => {
  const mockSecret: InferenceSecret = {
    name: 'my-secret',
  };

  server.use(
    http.post(cfg.api.inference.secrets, () => HttpResponse.json(mockSecret))
  );

  renderCreateInferenceSecret();

  const user = userEvent.setup();

  const apiKeyInput = screen.getByLabelText(/API Key/i);
  const nameInput = screen.getByLabelText(/Name/i);

  await user.type(apiKeyInput, 'sk-test-api-key');
  await user.type(nameInput, 'my-secret');

  const createButton = screen.getByRole('button', { name: /Create/i });
  await user.click(createButton);

  await waitFor(() => {
    expect(
      screen.queryByText('Create Inference Secret')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when creation fails', async () => {
  server.use(
    http.post(cfg.api.inference.secrets, () =>
      HttpResponse.json(
        { error: { message: 'Secret already exists' } },
        { status: 400 }
      )
    )
  );

  renderCreateInferenceSecret();

  const user = userEvent.setup();

  await user.type(screen.getByLabelText(/API Key/i), 'sk-test-api-key');
  await user.type(screen.getByLabelText(/Name/i), 'my-secret');
  await user.click(screen.getByRole('button', { name: /Create/i }));

  await waitFor(() => {
    expect(screen.getByText(/Secret already exists/i)).toBeInTheDocument();
  });
});

test('cancel button is present and clickable', async () => {
  renderCreateInferenceSecret();

  const cancelButton = screen.getByRole('button', { name: /Cancel/i });

  expect(cancelButton).toBeInTheDocument();
  expect(cancelButton).toBeEnabled();
});

test('create button shows correct label', () => {
  renderCreateInferenceSecret();

  const createButton = screen.getByRole('button', { name: /Create/i });

  expect(createButton).toHaveTextContent('Create');
});

function renderCreateInferenceSecret() {
  return render(
    <MemoryRouter initialEntries={['/#new-secret']}>
      <SessionSummariesManagementProvider />
    </MemoryRouter>
  );
}
