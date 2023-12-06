import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { render, fireEvent, waitFor, screen } from 'design/utils/testing';
import history from 'teleport/services/history';
import MfaService from 'teleport/services/mfa';

import { act } from '@testing-library/react';

import cfg from 'e-teleport/config';
import RecoveryService from 'e-teleport/services/recovery';

import Recovery from '../Recovery';

const startToken = {
  username: 'joe@example.com',
  isApproved: false,
  qrCode: '',
  id: 'startTokenId',
};

const approvedToken = {
  username: 'joe@example.com',
  isApproved: true,
  qrCode: '',
  id: 'approvedTokenId',
};

const recoveryCodes = {
  codes: [
    'tele-recovery-code-1',
    'tele-recovery-code-2',
    'tele-recovery-code-3',
  ],
  createdDate: new Date('2019-08-30T00:00:00.00Z'),
};

const route1VerifyWithStartToken = '/web/recovery/steps/startTokenId/verify';
const route2NewPasswordWithApprovedToken =
  '/web/recovery/steps/approvedTokenId/new/password';
const route2NewDeviceWithApprovedToken =
  '/web/recovery/steps/approvedTokenId/new/device';
const route3DevicesWithApprovedToken =
  '/web/recovery/steps/approvedTokenId/devices';
const routeNewCodesWithApprovedToken =
  '/web/recovery/steps/approvedTokenId/codes';

describe('all recovery flows should show correct screens', () => {
  const setup = () => {
    const mockHistory = createMemoryHistory({
      initialEntries: ['/web/recovery/startTokenId'],
    });

    jest.spyOn(history, 'push').mockImplementation();

    jest.spyOn(history, 'replace').mockImplementation();

    jest
      .spyOn(RecoveryService.prototype, 'setNewTotpDeviceOrPassword')
      .mockResolvedValue({});

    jest
      .spyOn(RecoveryService.prototype, 'generateRecoveryCodes')
      .mockResolvedValue(recoveryCodes);

    jest
      .spyOn(MfaService.prototype, 'fetchDevicesWithToken')
      .mockResolvedValue([
        {
          id: '1',
          description: 'Authenticator App',
          name: 'iphone 12',
          registeredDate: new Date(1626464043),
          lastUsedDate: new Date(1626472652),
        },
        {
          id: '2',
          description: 'Hardware Key',
          name: 'solokey',
          registeredDate: new Date(1623722252),
          lastUsedDate: new Date(1623981452),
        },
      ]);

    render(
      <Router history={mockHistory}>
        <Recovery />
      </Router>
    );

    return { mockHistory };
  };

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('new password using otp', async () => {
    const { mockHistory } = setup();

    jest.spyOn(cfg.oss, 'getAuth2faType').mockReturnValue('otp');
    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...startToken,
        isRecoverPassword: true,
      });
    jest.spyOn(RecoveryService.prototype, 'verifyUser').mockResolvedValue({
      ...approvedToken,
      isRecoverPassword: true,
    });

    act(() => mockHistory.replace(route1VerifyWithStartToken));

    await waitFor(() => {
      expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);
    });

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: true,
      });

    const tokenField = screen.getByPlaceholderText('123 456');

    fireEvent.change(tokenField, { target: { value: '321321' } });

    fireEvent.click(screen.getByText('Continue'));
    act(() => mockHistory.push(route2NewPasswordWithApprovedToken));
    await waitFor(() => {
      expect(RecoveryService.prototype.verifyUser).toHaveBeenCalledWith({
        tokenId: startToken.id,
        username: startToken.username,
        secondFactorToken: '321321',
      });
    });

    expect(history.push).toHaveBeenLastCalledWith(
      route2NewPasswordWithApprovedToken
    );

    const newPasswordField = screen.getByPlaceholderText('Password');
    const newConfirmPasswordField =
      screen.getByPlaceholderText('Confirm Password');

    fireEvent.change(newPasswordField, { target: { value: 'password123' } });
    fireEvent.change(newConfirmPasswordField, {
      target: { value: 'password123' },
    });

    fireEvent.click(screen.getByText('Continue'));
    act(() => mockHistory.push(routeNewCodesWithApprovedToken));

    await waitFor(() => {
      expect(
        RecoveryService.prototype.setNewTotpDeviceOrPassword
      ).toHaveBeenCalledWith({
        tokenId: approvedToken.id,
        password: 'password123',
      });
    });

    expect(history.push).toHaveBeenLastCalledWith(
      routeNewCodesWithApprovedToken
    );

    expect(
      RecoveryService.prototype.generateRecoveryCodes
    ).toHaveBeenCalledWith(approvedToken.id);

    expect(screen.getByText('New Backup & Recovery Codes')).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
    expect(
      screen.getByText(recoveryCodes.createdDate.toString(), { exact: false })
    ).toBeInTheDocument();
  });

  test('new otp device using password', async () => {
    const { mockHistory } = setup();

    jest.spyOn(cfg.oss, 'getAuth2faType').mockReturnValue('otp');
    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...startToken,
        isRecoverPassword: false,
      });
    jest.spyOn(RecoveryService.prototype, 'verifyUser').mockResolvedValue({
      ...approvedToken,
      isRecoverPassword: false,
    });

    act(() => mockHistory.replace(route1VerifyWithStartToken));

    await waitFor(() => {
      expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);
    });

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: false,
      });

    const passwordField = screen.getByPlaceholderText('Password');

    fireEvent.change(passwordField, { target: { value: 'password123' } });

    fireEvent.click(screen.getByText('Continue'));
    act(() => mockHistory.push(route2NewDeviceWithApprovedToken));

    expect(RecoveryService.prototype.verifyUser).toHaveBeenCalledWith({
      tokenId: startToken.id,
      username: startToken.username,
      password: 'password123',
    });

    await waitFor(() => {
      expect(history.push).toHaveBeenLastCalledWith(
        route2NewDeviceWithApprovedToken
      );
    });

    const newTokenField = screen.getByPlaceholderText('123 456');
    const deviceNameField = screen.getByPlaceholderText(/name/i);

    fireEvent.change(newTokenField, { target: { value: '321321' } });
    fireEvent.change(deviceNameField, { target: { value: 'backup' } });

    fireEvent.click(screen.getByText('Continue'));
    act(() => mockHistory.push(route3DevicesWithApprovedToken));

    expect(
      RecoveryService.prototype.setNewTotpDeviceOrPassword
    ).toHaveBeenCalledWith({
      tokenId: approvedToken.id,
      otpCode: '321321',
      deviceName: 'backup',
    });

    await waitFor(() => {
      expect(history.push).toHaveBeenLastCalledWith(
        route3DevicesWithApprovedToken
      );
    });

    expect(MfaService.prototype.fetchDevicesWithToken).toHaveBeenCalledWith(
      approvedToken.id
    );

    expect(
      screen.getByText(/take a look at your enrolled devices below/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/iphone 12/i)).toBeInTheDocument();
    expect(screen.getByText(/solokey/i)).toBeInTheDocument();

    fireEvent.click(screen.getByText('Continue'));
    act(() => mockHistory.push(routeNewCodesWithApprovedToken));

    expect(history.push).toHaveBeenLastCalledWith(
      routeNewCodesWithApprovedToken
    );

    await waitFor(() => {
      expect(
        RecoveryService.prototype.generateRecoveryCodes
      ).toHaveBeenCalledWith(approvedToken.id);
    });

    expect(screen.getByText('New Backup & Recovery Codes')).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
    expect(
      screen.getByText(recoveryCodes.createdDate.toString(), { exact: false })
    ).toBeInTheDocument();
  });
});
