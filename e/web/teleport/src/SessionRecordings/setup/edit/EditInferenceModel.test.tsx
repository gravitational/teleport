import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';

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
beforeEach(() => {
  testQueryClient.clear();
});
afterEach(() => {
  server.resetHandlers();
  testQueryClient.clear();
});
afterAll(() => server.close());

const mockModel: InferenceModel = {
  name: 'my-model',
  openai: {
    modelId: 'gpt-4',
    api_key_secret_ref: 'some-secret',
    base_url: 'https://api.example.com',
  },
};

test('renders the edit dialog with correct title', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();
  renderEditInferenceModel();

  expect(
    await screen.findByText('Editing Inference Model: my-model')
  ).toBeInTheDocument();
});

test('renders the form fields with existing values', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();
  renderEditInferenceModel();

  expect(await screen.findByText(/Choose model provider/i)).toBeInTheDocument();
  expect(screen.getByDisplayValue('gpt-4')).toBeInTheDocument();
  expect(
    screen.getByDisplayValue('https://api.example.com')
  ).toBeInTheDocument();
});

test('does not render the name field in edit mode', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();
  renderEditInferenceModel();

  expect(
    await screen.findByText('Editing Inference Model: my-model')
  ).toBeInTheDocument();
  expect(
    screen.queryByPlaceholderText('my-inference-model')
  ).not.toBeInTheDocument();
});

test('successfully updates an inference model', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();

  let updateCalled = false;
  server.use(
    http.put(cfg.api.inference.model, () => {
      updateCalled = true;
      return HttpResponse.json(mockModel);
    }),
    http.post(cfg.api.inference.testModel, () =>
      HttpResponse.json({ success: true })
    )
  );

  renderEditInferenceModel();

  const user = userEvent.setup();
  const testButton = await screen.findByRole('button', {
    name: /Test Connection/i,
  });
  await user.click(testButton);

  expect(await screen.findByRole('button', { name: /Update/i })).toBeEnabled();

  await user.click(screen.getByRole('button', { name: /Update/i }));
  await waitFor(() => {
    expect(updateCalled).toBe(true);
  });
  await waitFor(() => {
    expect(
      screen.queryByText('Editing Inference Model: my-model')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when update fails', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();

  server.use(
    http.put(cfg.api.inference.model, () =>
      HttpResponse.json(
        { error: { message: 'Failed to update model' } },
        { status: 400 }
      )
    ),
    http.post(cfg.api.inference.testModel, () =>
      HttpResponse.json({ success: true })
    )
  );

  renderEditInferenceModel();

  const user = userEvent.setup();
  const testButton = await screen.findByRole('button', {
    name: /Test Connection/i,
  });
  await user.click(testButton);

  expect(await screen.findByRole('button', { name: /Update/i })).toBeEnabled();
  await user.click(screen.getByRole('button', { name: /Update/i }));
  await waitFor(() => {
    expect(screen.getByText(/Failed to update model/i)).toBeInTheDocument();
  });
});

test('cancel button is present and clickable', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();
  renderEditInferenceModel();

  expect(
    await screen.findByRole('button', { name: /Cancel/i })
  ).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /Cancel/i })).toBeEnabled();
});

test('update button shows correct label', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();
  renderEditInferenceModel();

  expect(
    await screen.findByRole('button', { name: /Update/i })
  ).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /Update/i })).toHaveTextContent(
    'Update'
  );
});

test('delete button is present', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();
  renderEditInferenceModel();

  expect(
    await screen.findByRole('button', { name: /Delete Inference Model/i })
  ).toBeInTheDocument();
});

test('successfully deletes an inference model', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();

  server.use(
    http.delete(cfg.api.inference.model, () => HttpResponse.json({})),
    http.get(cfg.api.inference.models, () =>
      HttpResponse.json({ items: [], totalCount: 0 })
    )
  );

  renderEditInferenceModel();

  const user = userEvent.setup();
  const deleteButton = await screen.findByRole('button', {
    name: 'Delete Inference Model',
  });

  await user.click(deleteButton);

  const confirmButton = await screen.findByRole('button', {
    name: 'Yes, Delete',
  });

  await user.click(confirmButton);
  await waitFor(() => {
    expect(
      screen.queryByText('Editing Inference Model: my-model')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when delete fails', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();

  server.use(
    http.delete(cfg.api.inference.model, () =>
      HttpResponse.json(
        { error: { message: 'Failed to delete model' } },
        { status: 400 }
      )
    )
  );

  renderEditInferenceModel();

  const user = userEvent.setup();
  const deleteButton = await screen.findByRole('button', {
    name: 'Delete Inference Model',
  });
  await user.click(deleteButton);

  const confirmButton = await screen.findByRole('button', {
    name: 'Yes, Delete',
  });
  await user.click(confirmButton);
  expect(
    await screen.findByText(/Failed to delete model/i)
  ).toBeInTheDocument();
});

test('the test connection button is present', async () => {
  mockGetInferenceModel();
  mockListInferenceSecrets();
  renderEditInferenceModel();
  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: /Test Connection/i })
    ).toBeInTheDocument();
  });
});

function renderEditInferenceModel() {
  return render(
    <MemoryRouter initialEntries={['/#edit-model:my-model']}>
      <SessionSummariesManagementProvider />
    </MemoryRouter>
  );
}

function mockGetInferenceModel() {
  server.use(
    http.get(cfg.api.inference.model, () => HttpResponse.json(mockModel))
  );
}

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
