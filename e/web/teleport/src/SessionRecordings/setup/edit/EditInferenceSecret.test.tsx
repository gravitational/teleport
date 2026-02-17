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
import type { InferenceSecret } from 'e-teleport/services/inference/types';
import { SessionSummariesManagementProvider } from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';

const server = setupServer();

beforeAll(() => server.listen());
afterEach(() => {
  server.resetHandlers();
  testQueryClient.clear();
});
afterAll(() => server.close());

test('renders the edit dialog with correct title', () => {
  renderEditInferenceSecret();

  expect(
    screen.getByText('Editing Inference Secret: my-secret')
  ).toBeInTheDocument();
});

test('renders the form fields', () => {
  renderEditInferenceSecret();

  expect(screen.getByLabelText(/Update API Key/i)).toBeInTheDocument();
});

test('does not render the name field in edit mode', () => {
  renderEditInferenceSecret();

  expect(screen.queryByLabelText(/^Name$/i)).not.toBeInTheDocument();
});

test('update button is disabled when form is invalid', () => {
  renderEditInferenceSecret();

  const updateButton = screen.getByRole('button', { name: /Update/i });

  expect(updateButton).toBeDisabled();
});

test('update button is enabled when form is valid', async () => {
  renderEditInferenceSecret();

  const user = userEvent.setup();

  const apiKeyInput = screen.getByLabelText(/Update API Key/i);

  await user.type(apiKeyInput, 'sk-new-api-key');

  await waitFor(() => {
    expect(screen.getByRole('button', { name: /Update/i })).toBeEnabled();
  });
});

test('successfully updates an inference secret', async () => {
  const mockSecret: InferenceSecret = {
    name: 'my-secret',
  };

  let updateCalled = false;

  server.use(
    http.put(cfg.api.inference.secret, () => {
      updateCalled = true;
      return HttpResponse.json(mockSecret);
    })
  );

  renderEditInferenceSecret();

  const user = userEvent.setup();

  const apiKeyInput = screen.getByLabelText(/Update API Key/i);

  await user.type(apiKeyInput, 'sk-new-api-key');

  const updateButton = screen.getByRole('button', { name: /Update/i });
  await user.click(updateButton);

  await waitFor(() => {
    expect(updateCalled).toBe(true);
  });

  await waitFor(() => {
    expect(
      screen.queryByText('Editing Inference Secret: my-secret')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when update fails', async () => {
  server.use(
    http.put(cfg.api.inference.secret, () =>
      HttpResponse.json(
        { error: { message: 'Failed to update secret' } },
        { status: 400 }
      )
    )
  );

  renderEditInferenceSecret();

  const user = userEvent.setup();

  await user.type(screen.getByLabelText(/Update API Key/i), 'sk-new-api-key');
  await user.click(screen.getByRole('button', { name: /Update/i }));

  await waitFor(() => {
    expect(screen.getByText(/Failed to update secret/i)).toBeInTheDocument();
  });
});

test('cancel button is present and clickable', async () => {
  renderEditInferenceSecret();

  const cancelButton = screen.getByRole('button', { name: /Cancel/i });

  expect(cancelButton).toBeInTheDocument();
  expect(cancelButton).toBeEnabled();
});

test('update button shows correct label', () => {
  renderEditInferenceSecret();

  const updateButton = screen.getByRole('button', { name: /Update/i });

  expect(updateButton).toHaveTextContent('Update');
});

test('delete button is present', () => {
  renderEditInferenceSecret();

  const deleteButton = screen.getByRole('button', {
    name: /Delete Inference Secret/i,
  });

  expect(deleteButton).toBeInTheDocument();
});

test('successfully deletes an inference secret', async () => {
  server.use(
    http.delete(cfg.api.inference.secret, () => HttpResponse.json({})),
    http.get(cfg.api.inference.secrets, () =>
      HttpResponse.json({ secrets: [], totalCount: 0 })
    )
  );

  renderEditInferenceSecret();

  const user = userEvent.setup();

  const deleteButton = screen.getByRole('button', {
    name: 'Delete Inference Secret',
  });

  await user.click(deleteButton);

  const confirmButton = await screen.findByRole('button', {
    name: 'Yes, Delete',
  });

  await user.click(confirmButton);

  await waitFor(() => {
    expect(
      screen.queryByText('Editing Inference Secret: my-secret')
    ).not.toBeInTheDocument();
  });
});

test('displays error message when delete fails', async () => {
  server.use(
    http.delete(cfg.api.inference.secret, () =>
      HttpResponse.json(
        { error: { message: 'Failed to delete secret' } },
        { status: 400 }
      )
    )
  );

  renderEditInferenceSecret();

  const user = userEvent.setup();

  const deleteButton = screen.getByRole('button', {
    name: 'Delete Inference Secret',
  });
  await user.click(deleteButton);

  const confirmButton = await screen.findByRole('button', {
    name: 'Yes, Delete',
  });
  await user.click(confirmButton);

  await waitFor(() => {
    expect(screen.getByText(/Failed to delete secret/i)).toBeInTheDocument();
  });
});

function renderEditInferenceSecret() {
  return render(
    <MemoryRouter initialEntries={['/#edit-secret:my-secret']}>
      <SessionSummariesManagementProvider />
    </MemoryRouter>
  );
}
