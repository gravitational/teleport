import { userEvent, render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';

import EmailPasswordResetDialog from './EmailPasswordResetDialog';

describe('reset dialog', () => {
  let ctx, onClose, onSubmit;
  beforeEach(() => {
    onClose = jest.fn();
    onSubmit = jest.fn(() => Promise.resolve([]));
    ctx = createTeleportContext() as any;
    ctx.cloudService = {
      sendTeleportCredentialReset: onSubmit,
    };
  });

  test('renders the form for an email-like user', async () => {
    render(
      <ContextProvider ctx={ctx}>
        <EmailPasswordResetDialog
          username={'alice@example.com'}
          onClose={onClose}
        />
      </ContextProvider>
    );

    expect(screen.getByText('Reset User Credentials')).toBeInTheDocument();

    // Submit button should make an API call
    const submitButton = screen.getByText('Send New Invite');
    expect(submitButton).toBeInTheDocument();
    expect(submitButton).toBeEnabled();

    await userEvent.click(submitButton);
    expect(onSubmit.mock.calls).toHaveLength(1);

    // Close button should now exist and call the close mock
    const closeButton = screen.getByText('Close');
    expect(closeButton).toBeInTheDocument();

    await userEvent.click(closeButton);
    expect(onClose.mock.calls).toHaveLength(1);
  });

  test('renders the form for a non-email-like user', async () => {
    render(
      <ContextProvider ctx={ctx}>
        <EmailPasswordResetDialog username={'alice'} onClose={onClose} />
      </ContextProvider>
    );

    expect(screen.getByText('Reset User Credentials')).toBeInTheDocument();

    // Submit button should exist but be disabled.
    const submitButton = screen.getByText('Send New Invite');
    expect(submitButton).toBeInTheDocument();
    expect(submitButton).toBeDisabled();

    await userEvent.click(submitButton);
    expect(onSubmit.mock.calls).toHaveLength(0);

    // Cancel should render and work properly.
    const cancelButton = screen.getByText('Cancel');
    expect(cancelButton).toBeInTheDocument();

    await userEvent.click(cancelButton);
    expect(onClose.mock.calls).toHaveLength(1);
  });
});
