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
import { SessionSummariesManagementProvider } from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';
import type {
  InferencePolicy,
  ListInferenceModelsResponse,
} from 'e-teleport/services/inference/types';

const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => {
  server.resetHandlers();
  testQueryClient.clear();
});
afterAll(() => server.close());

const mockPolicy: InferencePolicy = {
  name: 'my-policy',
  model: 'some-inference-model',
  kinds: ['k8s'],
};

test('renders the edit dialog with correct title', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  await waitFor(() => {
    expect(
      screen.getByText('Editing Inference Policy: my-policy')
    ).toBeInTheDocument();
  });
});

test('renders the form fields with existing values', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  await waitFor(() => {
    expect(screen.getByText(/Inference model to use/i)).toBeInTheDocument();
  });

  expect(screen.getByText('some-inference-model')).toBeInTheDocument();
  expect(screen.getByRole('checkbox', { name: /Kubernetes/i })).toBeChecked();
});

test('does not render the name field in edit mode', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  await waitFor(() => {
    expect(
      screen.getByText('Editing Inference Policy: my-policy')
    ).toBeInTheDocument();
  });

  expect(screen.queryByLabelText(/^Name$/i)).not.toBeInTheDocument();
});

test('update button is enabled when form is valid', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Update/i })).toBeEnabled();
  });
});

test('successfully updates an inference policy', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();

  let updateCalled = false;

  server.use(
    http.put(cfg.api.inference.policy, () => {
      updateCalled = true;
      return HttpResponse.json(mockPolicy);
    })
  );

  renderEditInferencePolicy();

  const user = userEvent.setup();

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Update/i })).toBeEnabled();
  });

  const modelSelect = screen.getByRole('combobox');
  await selectEvent.select(modelSelect, 'another-inference-model');

  const updateButton = screen.getByRole('button', { name: /Update/i });
  await user.click(updateButton);

  await waitFor(() => {
    expect(updateCalled).toBe(true);
  });

  await waitFor(() => {
    expect(
      screen.queryByText('Editing Inference Policy: my-policy')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when update fails', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();

  server.use(
    http.put(cfg.api.inference.policy, () =>
      HttpResponse.json(
        { error: { message: 'Failed to update policy' } },
        { status: 400 }
      )
    )
  );

  renderEditInferencePolicy();

  const user = userEvent.setup();

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Update/i })).toBeEnabled();
  });

  await user.click(screen.getByRole('button', { name: /Update/i }));

  await waitFor(() => {
    expect(screen.getByText(/Failed to update policy/i)).toBeInTheDocument();
  });
});

test('cancel button is present and clickable', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Cancel/i })).toBeInTheDocument();
  });

  expect(screen.getByRole('button', { name: /Cancel/i })).toBeEnabled();
});

test('update button shows correct label', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Update/i })).toBeInTheDocument();
  });

  expect(screen.getByRole('button', { name: /Update/i })).toHaveTextContent(
    'Update'
  );
});

test('delete button is present', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: /Delete Inference Policy/i })
    ).toBeInTheDocument();
  });
});

test('successfully deletes an inference policy', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();

  server.use(
    http.delete(cfg.api.inference.policy, () => HttpResponse.json({})),
    http.get(cfg.api.inference.policies, () =>
      HttpResponse.json({ items: [], totalCount: 0 })
    )
  );

  renderEditInferencePolicy();

  const user = userEvent.setup();

  const deleteButton = await screen.findByRole('button', {
    name: 'Delete Inference Policy',
  });

  await user.click(deleteButton);

  const confirmButton = await screen.findByRole('button', {
    name: 'Yes, Delete',
  });

  await user.click(confirmButton);

  await waitFor(() => {
    expect(
      screen.queryByText('Editing Inference Policy: my-policy')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when delete fails', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();

  server.use(
    http.delete(cfg.api.inference.policy, () =>
      HttpResponse.json(
        { error: { message: 'Failed to delete policy' } },
        { status: 400 }
      )
    )
  );

  renderEditInferencePolicy();

  const user = userEvent.setup();

  const deleteButton = await screen.findByRole('button', {
    name: 'Delete Inference Policy',
  });
  await user.click(deleteButton);

  const confirmButton = await screen.findByRole('button', {
    name: 'Yes, Delete',
  });
  await user.click(confirmButton);

  await waitFor(() => {
    expect(screen.getByText(/Failed to delete policy/i)).toBeInTheDocument();
  });
});

test('can change session types', async () => {
  mockGetInferencePolicy();
  mockListInferenceModels();
  renderEditInferencePolicy();

  const user = userEvent.setup();

  await waitFor(() => {
    expect(screen.getByRole('checkbox', { name: /Kubernetes/i })).toBeChecked();
  });

  await user.click(screen.getByRole('checkbox', { name: /Database/i }));

  expect(screen.getByRole('checkbox', { name: /Database/i })).toBeChecked();
  expect(screen.getByRole('checkbox', { name: /Kubernetes/i })).toBeChecked();
});

function renderEditInferencePolicy() {
  return render(
    <MemoryRouter initialEntries={['/#edit-policy:my-policy']}>
      <SessionSummariesManagementProvider />
    </MemoryRouter>
  );
}

function mockGetInferencePolicy() {
  server.use(
    http.get(cfg.api.inference.policy, () => HttpResponse.json(mockPolicy))
  );
}

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
