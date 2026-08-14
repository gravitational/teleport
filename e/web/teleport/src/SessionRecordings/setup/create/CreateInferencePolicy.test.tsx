import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';
import selectEvent from 'react-select-event';

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
import type {
  InferencePolicy,
  ListInferenceModelsResponse,
} from 'e-teleport/services/inference/types';
import { SessionSummariesManagementProvider } from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';

enableMswServer();

afterEach(() => {
  testQueryClient.clear();
});

function mockListInferenceModels() {
  const mockModelsResponse: ListInferenceModelsResponse = {
    items: [
      { name: 'some-inference-model' },
      { name: 'another-inference-model' },
    ],
  };

  server.use(
    http.get(cfg.api.inference.models, () =>
      HttpResponse.json(mockModelsResponse)
    )
  );
}

test('renders the create dialog with correct title', async () => {
  mockListInferenceModels();
  renderCreateInferencePolicy();

  expect(screen.getByText('Create Inference Policy')).toBeInTheDocument();
});

test('renders the form fields', async () => {
  mockListInferenceModels();
  renderCreateInferencePolicy();

  expect(screen.getByText(/Inference model to use/i)).toBeInTheDocument();
  expect(
    screen.getByText(/What types of sessions should be summarized/i)
  ).toBeInTheDocument();
  expect(screen.getByLabelText(/Name/i)).toBeInTheDocument();
});

test('create button is disabled when form is invalid', async () => {
  mockListInferenceModels();
  renderCreateInferencePolicy();

  const createButton = screen.getByRole('button', { name: /Create/i });

  expect(createButton).toBeDisabled();
});

test('create button is enabled when form is valid', async () => {
  mockListInferenceModels();
  renderCreateInferencePolicy();

  const user = userEvent.setup();

  const modelSelect = screen.getByRole('combobox');
  await selectEvent.select(modelSelect, 'some-inference-model');

  await user.click(screen.getByRole('checkbox', { name: /Kubernetes/i }));

  await user.type(screen.getByLabelText(/Name/i), 'my-policy');

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Create/i })).toBeEnabled();
  });
});

test('successfully creates an inference policy', async () => {
  mockListInferenceModels();

  const mockPolicy: InferencePolicy = {
    name: 'my-policy',
    model: 'some-inference-model',
    kinds: ['k8s'],
  };

  server.use(
    http.post(cfg.api.inference.policies, () => HttpResponse.json(mockPolicy))
  );

  renderCreateInferencePolicy();

  const user = userEvent.setup();

  const modelSelect = screen.getByRole('combobox');

  await selectEvent.select(modelSelect, 'some-inference-model');

  await user.click(screen.getByRole('checkbox', { name: /Kubernetes/i }));
  await user.type(screen.getByLabelText(/Name/i), 'my-policy');

  const createButton = screen.getByRole('button', { name: /Create/i });

  await user.click(createButton);

  await waitFor(() => {
    expect(
      screen.queryByText('Create Inference Policy')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when creation fails', async () => {
  mockListInferenceModels();

  server.use(
    http.post(cfg.api.inference.policies, () =>
      HttpResponse.json(
        { error: { message: 'Policy already exists' } },
        { status: 400 }
      )
    )
  );

  renderCreateInferencePolicy();

  const user = userEvent.setup();

  const modelSelect = screen.getByRole('combobox');

  await selectEvent.select(modelSelect, 'some-inference-model');

  await user.click(screen.getByRole('checkbox', { name: /Kubernetes/i }));
  await user.type(screen.getByLabelText(/Name/i), 'my-policy');
  await user.click(screen.getByRole('button', { name: /Create/i }));

  await waitFor(() => {
    expect(screen.getByText(/Policy already exists/i)).toBeInTheDocument();
  });
});

test('cancel button is present and clickable', async () => {
  mockListInferenceModels();
  renderCreateInferencePolicy();

  const cancelButton = screen.getByRole('button', { name: /Cancel/i });

  expect(cancelButton).toBeInTheDocument();
  expect(cancelButton).toBeEnabled();
});

test('create button shows correct label', async () => {
  mockListInferenceModels();
  renderCreateInferencePolicy();

  const createButton = screen.getByRole('button', { name: /Create/i });

  expect(createButton).toHaveTextContent('Create');
});

test('can select multiple session types', async () => {
  mockListInferenceModels();
  renderCreateInferencePolicy();

  const user = userEvent.setup();

  await user.click(screen.getByRole('checkbox', { name: /Kubernetes/i }));
  await user.click(screen.getByRole('checkbox', { name: /Database/i }));
  await user.click(screen.getByRole('checkbox', { name: /SSH/i }));

  expect(screen.getByRole('checkbox', { name: /Kubernetes/i })).toBeChecked();
  expect(screen.getByRole('checkbox', { name: /Database/i })).toBeChecked();
  expect(screen.getByRole('checkbox', { name: /SSH/i })).toBeChecked();
});

test('shows terms and conditions when using Teleport Cloud', async () => {
  cfg.oss.isCloud = true;
  mockListInferenceModels();
  renderCreateInferencePolicy();

  expect(
    screen.getByRole('checkbox', { name: /I have read and agree/i })
  ).toBeInTheDocument();
  cfg.oss.isCloud = false;
});

test('create button is disabled without accepting terms', async () => {
  cfg.oss.isCloud = true;
  mockListInferenceModels();
  renderCreateInferencePolicy();

  const user = userEvent.setup();

  await user.click(screen.getByRole('checkbox', { name: /Kubernetes/i }));
  await user.type(screen.getByLabelText(/Name/i), 'my-policy');

  expect(screen.getByRole('button', { name: /Create/i })).toBeDisabled();
  cfg.oss.isCloud = false;
});

test('create button is enabled after accepting terms and filling form', async () => {
  cfg.oss.isCloud = true;
  mockListInferenceModels();
  renderCreateInferencePolicy();

  const user = userEvent.setup();

  await user.click(screen.getByRole('checkbox', { name: /Kubernetes/i }));
  await user.type(screen.getByLabelText(/Name/i), 'my-policy');
  await user.click(
    screen.getByRole('checkbox', { name: /I have read and agree/i })
  );

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Create/i })).toBeEnabled();
  });
  cfg.oss.isCloud = false;
});

function renderCreateInferencePolicy() {
  return render(
    <MemoryRouter initialEntries={['/#new-policy']}>
      <SessionSummariesManagementProvider />
    </MemoryRouter>
  );
}
