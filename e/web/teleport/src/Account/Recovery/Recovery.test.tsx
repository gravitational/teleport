import React from 'react';
import { render, fireEvent, waitFor, screen } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import AuthService from 'teleport/services/auth';

import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

import Recovery from './Recovery';

const privilegeToken = 'privilegeToken123';

describe('recovery dashboard testing', () => {
  const setup = () => {
    const ctx = new TeleportContextE();
    ctx.storeUser.setState({ username: 'joe@example.com' });

    jest
      .spyOn(AuthService, 'createPrivilegeTokenWithTotp')
      .mockResolvedValue(privilegeToken);

    jest
      .spyOn(AuthService, 'createPrivilegeTokenWithWebauthn')
      .mockResolvedValue(privilegeToken);

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

    render(
      <ContextProvider ctx={ctx}>
        <Recovery />
      </ContextProvider>
    );

    return { ctx };
  };

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('generating new codes with totp', async () => {
    const { ctx } = setup();

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    fireEvent.click(await screen.findByText('Generate new recovery codes'));

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    const reAuthMfaSelectEl = screen
      .getByTestId('mfa-select')
      .querySelector('input');
    fireEvent.keyDown(reAuthMfaSelectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getAllByText(/authenticator app/i)[0]);

    const tokenField = screen.getByPlaceholderText('123 456');
    fireEvent.change(tokenField, { target: { value: '321321' } });

    fireEvent.click(screen.getByText('Continue'));

    expect(AuthService.createPrivilegeTokenWithTotp).toHaveBeenCalledWith(
      '321321'
    );

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
    const { ctx } = setup();

    jest.spyOn(cfg.oss, 'getPreferredMfaType').mockReturnValue('webauthn');

    expect(ctx.recoveryService.fetchRecoveryCodesMetadata).toHaveBeenCalled();

    fireEvent.click(await screen.findByText('Generate new recovery codes'));

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Continue'));

    expect(AuthService.createPrivilegeTokenWithWebauthn).toHaveBeenCalled();

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
});
