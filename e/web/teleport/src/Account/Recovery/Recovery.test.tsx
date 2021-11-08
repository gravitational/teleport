import React from 'react';
import { render, fireEvent, wait, screen } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import AuthService from 'teleport/services/auth';
import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';
import Recovery from './Recovery';

const privilegeToken = 'privilegeToken123';

describe('recovery dashboard testing', () => {
  let renderRecoveryDashboard;
  let ctx: TeleportContextE;

  beforeEach(() => {
    ctx = new TeleportContextE();

    ctx.storeUser.setState({ username: 'joe@example.com' });

    renderRecoveryDashboard = () =>
      render(
        <ContextProvider ctx={ctx}>
          <Recovery />
        </ContextProvider>
      );

    jest
      .spyOn(AuthService, 'createPrivilegeTokenWithTotp')
      .mockResolvedValue(privilegeToken);

    jest
      .spyOn(AuthService, 'createPrivilegeTokenWithU2f')
      .mockResolvedValue(privilegeToken);

    jest
      .spyOn(AuthService, 'createPrivilegeTokenWithWebauthn')
      .mockResolvedValue(privilegeToken);

    jest
      .spyOn(ctx.recoveryService, 'fetchRecoveryCodesMetadata')
      .mockResolvedValue({ createdDate: new Date() });

    jest
      .spyOn(ctx.recoveryService, 'generateRecoveryCodes')
      .mockResolvedValue([
        'tele-recovery-code-1',
        'tele-recovery-code-2',
        'tele-recovery-code-3',
      ]);

    jest.spyOn(cfg.oss, 'getAuth2faType').mockReturnValue('on');

    jest.spyOn(cfg.oss, 'getPreferredMfaType').mockReturnValue('u2f');
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('generating new codes with totp', async () => {
    await wait(() => renderRecoveryDashboard());

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    await wait(() =>
      fireEvent.click(screen.getByText('Generate new recovery codes'))
    );

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    const reAuthMfaSelectEl = screen
      .getByTestId('mfa-select')
      .querySelector('input');
    fireEvent.keyDown(reAuthMfaSelectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getAllByText(/authenticator app/i)[0]);

    const tokenField = screen.getByPlaceholderText('123 456');
    fireEvent.change(tokenField, { target: { value: '321321' } });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
    });

    expect(AuthService.createPrivilegeTokenWithTotp).toHaveBeenCalledWith(
      '321321'
    );

    expect(ctx.recoveryService.generateRecoveryCodes).toHaveBeenCalledWith(
      privilegeToken
    );

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    expect(screen.getByText('New Backup & Recovery Codes')).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
  });

  test('generating new codes with u2f', async () => {
    await wait(() => renderRecoveryDashboard());

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    fireEvent.click(screen.getByText('Generate new recovery codes'));

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
    });

    expect(AuthService.createPrivilegeTokenWithU2f).toHaveBeenCalled();

    expect(ctx.recoveryService.generateRecoveryCodes).toHaveBeenCalledWith(
      privilegeToken
    );

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    expect(screen.getByText('New Backup & Recovery Codes')).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
  });

  test('generating new codes with webauthn', async () => {
    jest.spyOn(cfg.oss, 'getPreferredMfaType').mockReturnValue('webauthn');

    await wait(() => renderRecoveryDashboard());

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    fireEvent.click(screen.getByText('Generate new recovery codes'));

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
    });

    expect(AuthService.createPrivilegeTokenWithWebauthn).toHaveBeenCalled();

    expect(ctx.recoveryService.generateRecoveryCodes).toHaveBeenCalledWith(
      privilegeToken
    );

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    expect(screen.getByText('New Backup & Recovery Codes')).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
  });
});
