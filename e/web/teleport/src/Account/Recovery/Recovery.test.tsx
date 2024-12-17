import { render, fireEvent, waitFor, screen } from 'design/utils/testing';
import { ContextProvider } from 'teleport';

import auth from 'teleport/services/auth/auth';

import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

import Recovery from './Recovery';

const privilegeToken = 'privilegeToken123';

describe('recovery dashboard testing', () => {
  let addNotification;
  let ctx;

  beforeEach(() => {
    ctx = new TeleportContextE();
    ctx.storeUser.setState({ username: 'joe@example.com' });

    jest.spyOn(auth, 'getMfaChallenge').mockResolvedValueOnce({
      totpChallenge: true,
      webauthnPublicKey: {} as PublicKeyCredentialRequestOptions,
    });

    jest.spyOn(auth, 'getMfaChallengeResponse').mockResolvedValueOnce({});

    jest.spyOn(auth, 'createPrivilegeToken').mockResolvedValue(privilegeToken);

    jest
      .spyOn(ctx.recoveryService, 'fetchRecoveryCodesMetadata')
      .mockResolvedValue({ createdDate: new Date('2019-08-30T11:00:00.00Z') });

    jest.spyOn(ctx.recoveryService, 'generateRecoveryCodes').mockResolvedValue({
      codes: [
        'tele-recovery-code-1',
        'tele-recovery-code-2',
        'tele-recovery-code-3',
      ],
      createdDate: new Date('2019-08-30T11:00:00.00Z'),
    });

    jest.spyOn(cfg.oss, 'getAuth2faType').mockReturnValue('on');

    addNotification = jest.fn();
  });

  const renderRecovery = () =>
    render(
      <ContextProvider ctx={ctx}>
        <Recovery addNotification={addNotification} />
      </ContextProvider>
    );

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('generating new codes with totp', async () => {
    renderRecovery();

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    fireEvent.click(await screen.findByText('Generate new recovery codes'));

    await waitFor(() => {
      expect(screen.getByText('Verify your identity')).toBeInTheDocument();
    });

    const reAuthMfaSelectEl = screen
      .getByTestId('mfa-select')
      .querySelector('input');
    fireEvent.keyDown(reAuthMfaSelectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getAllByText(/authenticator app/i)[0]);

    const tokenField = screen.getByPlaceholderText('123 456');
    fireEvent.change(tokenField, { target: { value: '321321' } });

    fireEvent.click(screen.getByText('Continue'));

    await waitFor(() => {
      expect(auth.createPrivilegeToken).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(ctx.recoveryService.generateRecoveryCodes).toHaveBeenCalledWith(
        privilegeToken
      );
    });

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    expect(screen.getByText('New Backup & Recovery Codes')).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
  });

  test('generating new codes with webauthn', async () => {
    renderRecovery();

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    fireEvent.click(await screen.findByText('Generate new recovery codes'));

    await waitFor(() => {
      expect(screen.getByText('Verify your identity')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Continue'));

    await waitFor(() => {
      expect(auth.createPrivilegeToken).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(ctx.recoveryService.generateRecoveryCodes).toHaveBeenCalledWith(
        privilegeToken
      );
    });

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    expect(screen.getByText('New Backup & Recovery Codes')).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
  });

  test('fetches metadata, shows informational text and last generation date', async () => {
    renderRecovery();

    await waitFor(() => {
      expect(
        screen.getByText('Recovery codes were last generated on:')
      ).toBeInTheDocument();
    });
    expect(screen.getByText('8/30/2019')).toBeInTheDocument();
    expect(
      screen.getByText(
        /When you generate new recovery codes, your old ones will no longer work/i
      )
    ).toBeVisible();
  });

  test('shows a message when there are no recovery codes', async () => {
    jest
      .spyOn(ctx.recoveryService, 'fetchRecoveryCodesMetadata')
      .mockResolvedValue({});
    renderRecovery();
    await waitFor(() => {
      expect(screen.getByTestId('indicator-wrapper')).not.toBeVisible();
    });

    expect(
      screen.getByText(/Recovery codes are one-time use passcodes/i)
    ).toBeVisible();
  });

  test('shows a message when username is not an email', async () => {
    ctx.storeUser.setState({ username: 'foobar' });
    renderRecovery();

    expect(
      screen.getByText(
        /Account recovery is only available for local users with a valid email as their username/i
      )
    ).toBeVisible();
  });

  test('adds a notification when metadata fetch fails', async () => {
    ctx.recoveryService.fetchRecoveryCodesMetadata.mockRejectedValue(
      new Error('failed to fetch')
    );
    renderRecovery();

    await waitFor(() => {
      expect(addNotification).toHaveBeenCalledWith('error', 'failed to fetch');
    });
    expect(
      screen.queryByText('Recovery codes were last generated on:')
    ).not.toBeInTheDocument();
  });
});
