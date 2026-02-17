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
import type {
  InferenceModel,
  ListInferenceSecretsResponse,
} from 'e-teleport/services/inference/types';
import { SessionSummariesManagementProvider } from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';

const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => {
  server.resetHandlers();
  testQueryClient.clear();
});
afterAll(() => server.close());

function mockListInferenceSecrets() {
  const mockSecretsResponse: ListInferenceSecretsResponse = {
    items: [{ name: 'some-secret' }, { name: 'another-secret' }],
  };

  server.use(
    http.get(cfg.api.inference.secrets, () =>
      HttpResponse.json(mockSecretsResponse)
    )
  );
}

test('successfully creates an inference model', async () => {
  mockListInferenceSecrets();

  const mockModel: InferenceModel = {
    name: 'my-model',
    openai: {
      modelId: 'gpt-4',
      api_key_secret_ref: 'some-secret',
    },
  };

  server.use(
    http.post(cfg.api.inference.models, () => HttpResponse.json(mockModel)),
    http.post(cfg.api.inference.testModel, () =>
      HttpResponse.json({ success: true })
    )
  );

  renderCreateInferenceModel();

  const user = userEvent.setup();

  const secretSelect = screen.getByLabelText(
    /Secret to use for authentication/i
  );
  await selectEvent.select(secretSelect, 'some-secret');

  await user.type(screen.getByLabelText(/Model name/i), 'gpt-4');
  await user.type(
    screen.getByPlaceholderText('my-inference-model'),
    'my-model'
  );

  const testButton = screen.getByRole('button', { name: /Test Connection/i });
  await user.click(testButton);

  expect(await screen.findByRole('button', { name: /Create/i })).toBeEnabled();

  await user.click(screen.getByRole('button', { name: /Create/i }));

  await waitFor(() => {
    expect(
      screen.queryByText('Create Inference Model')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when creation fails', async () => {
  mockListInferenceSecrets();

  server.use(
    http.post(cfg.api.inference.models, () =>
      HttpResponse.json(
        { error: { message: 'Model already exists' } },
        { status: 400 }
      )
    ),
    http.post(cfg.api.inference.testModel, () =>
      HttpResponse.json({ success: true })
    )
  );

  renderCreateInferenceModel();

  const user = userEvent.setup();
  const secretSelect = screen.getByLabelText(
    /Secret to use for authentication/i
  );
  await selectEvent.select(secretSelect, 'some-secret');

  await user.type(screen.getByLabelText(/Model name/i), 'gpt-4');
  await user.type(
    screen.getByPlaceholderText('my-inference-model'),
    'my-model'
  );

  const testButton = screen.getByRole('button', { name: /Test Connection/i });
  await user.click(testButton);

  expect(await screen.findByRole('button', { name: /Create/i })).toBeEnabled();

  await user.click(screen.getByRole('button', { name: /Create/i }));
  expect(await screen.findByText(/Model already exists/i)).toBeInTheDocument();
});

function renderCreateInferenceModel() {
  return render(
    <MemoryRouter initialEntries={['/#new-model']}>
      <SessionSummariesManagementProvider />
    </MemoryRouter>
  );
}
