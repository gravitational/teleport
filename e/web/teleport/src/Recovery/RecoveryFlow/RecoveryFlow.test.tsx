import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { render, fireEvent, wait, screen } from 'design/utils/testing';
import history from 'teleport/services/history';
import MfaService from 'teleport/services/mfa';
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
  let mockHistory;
  let renderRecovery;

  beforeEach(() => {
    mockHistory = createMemoryHistory({
      initialEntries: ['/web/recovery/startTokenId'],
    });

    renderRecovery = () =>
      render(
        <Router history={mockHistory}>
          <Recovery />
        </Router>
      );

    jest.spyOn(history, 'push').mockImplementation();

    jest.spyOn(history, 'replace').mockImplementation();

    jest
      .spyOn(RecoveryService.prototype, 'setNewTotpDeviceOrPassword')
      .mockResolvedValue({});

    jest
      .spyOn(RecoveryService.prototype, 'setNewU2fDevice')
      .mockResolvedValue({});

    jest
      .spyOn(RecoveryService.prototype, 'generateRecoveryCodes')
      .mockResolvedValue([
        'tele-recovery-code-1',
        'tele-recovery-code-2',
        'tele-recovery-code-3',
      ]);

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
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('new password using otp', async () => {
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

    await wait(() => {
      renderRecovery();
      mockHistory.replace(route1VerifyWithStartToken);
    });

    expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: true,
      });

    const tokenField = screen.getByPlaceholderText('123 456');

    fireEvent.change(tokenField, { target: { value: '321321' } });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(route2NewPasswordWithApprovedToken);
    });

    expect(RecoveryService.prototype.verifyUser).toHaveBeenCalledWith({
      tokenId: startToken.id,
      username: startToken.username,
      secondFactorToken: '321321',
    });

    expect(history.push).toHaveBeenLastCalledWith(
      route2NewPasswordWithApprovedToken
    );

    const newPasswordField = screen.getByPlaceholderText('Password');
    const newConfirmPasswordField = screen.getByPlaceholderText(
      'Confirm Password'
    );

    fireEvent.change(newPasswordField, { target: { value: 'password123' } });
    fireEvent.change(newConfirmPasswordField, {
      target: { value: 'password123' },
    });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(routeNewCodesWithApprovedToken);
    });

    expect(
      RecoveryService.prototype.setNewTotpDeviceOrPassword
    ).toHaveBeenCalledWith({
      tokenId: approvedToken.id,
      password: 'password123',
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
  });

  test('new password with u2f', async () => {
    jest.spyOn(cfg.oss, 'getAuth2faType').mockReturnValue('u2f');
    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...startToken,
        isRecoverPassword: true,
      });
    jest
      .spyOn(RecoveryService.prototype, 'verifyUserWithU2f')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: true,
      });

    await wait(() => {
      renderRecovery();
      mockHistory.replace(route1VerifyWithStartToken);
    });

    expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: true,
      });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(route2NewPasswordWithApprovedToken);
    });

    expect(RecoveryService.prototype.verifyUserWithU2f).toHaveBeenCalledWith(
      startToken.id,
      startToken.username
    );

    expect(history.push).toHaveBeenLastCalledWith(
      route2NewPasswordWithApprovedToken
    );

    const newPasswordField = screen.getByPlaceholderText('Password');
    const newConfirmPasswordField = screen.getByPlaceholderText(
      'Confirm Password'
    );

    fireEvent.change(newPasswordField, { target: { value: 'password123' } });
    fireEvent.change(newConfirmPasswordField, {
      target: { value: 'password123' },
    });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(routeNewCodesWithApprovedToken);
    });

    expect(
      RecoveryService.prototype.setNewTotpDeviceOrPassword
    ).toHaveBeenCalledWith({
      tokenId: approvedToken.id,
      password: 'password123',
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
  });

  test('new otp device using password', async () => {
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

    await wait(() => {
      renderRecovery();
      mockHistory.replace(route1VerifyWithStartToken);
    });

    expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: false,
      });

    const passwordField = screen.getByPlaceholderText('Password');

    fireEvent.change(passwordField, { target: { value: 'password123' } });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(route2NewDeviceWithApprovedToken);
    });

    expect(RecoveryService.prototype.verifyUser).toHaveBeenCalledWith({
      tokenId: startToken.id,
      username: startToken.username,
      password: 'password123',
    });

    expect(history.push).toHaveBeenLastCalledWith(
      route2NewDeviceWithApprovedToken
    );

    const newTokenField = screen.getByPlaceholderText('123 456');
    const deviceNameField = screen.getByPlaceholderText(/name/i);

    fireEvent.change(newTokenField, { target: { value: '321321' } });
    fireEvent.change(deviceNameField, { target: { value: 'backup' } });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(route3DevicesWithApprovedToken);
    });

    expect(
      RecoveryService.prototype.setNewTotpDeviceOrPassword
    ).toHaveBeenCalledWith({
      tokenId: approvedToken.id,
      secondFactorToken: '321321',
      deviceName: 'backup',
    });

    expect(history.push).toHaveBeenLastCalledWith(
      route3DevicesWithApprovedToken
    );

    expect(MfaService.prototype.fetchDevicesWithToken).toHaveBeenCalledWith(
      approvedToken.id
    );

    expect(
      screen.getByText(/take a look at your enrolled devices below/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/iphone 12/i)).toBeInTheDocument();
    expect(screen.getByText(/solokey/i)).toBeInTheDocument();

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(routeNewCodesWithApprovedToken);
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
  });

  test('new u2f device using password', async () => {
    jest.spyOn(cfg.oss, 'getAuth2faType').mockReturnValue('u2f');
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

    await wait(() => {
      renderRecovery();
      mockHistory.replace(route1VerifyWithStartToken);
    });

    expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: false,
      });

    const passwordField = screen.getByPlaceholderText('Password');

    fireEvent.change(passwordField, { target: { value: 'password123' } });

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(route2NewDeviceWithApprovedToken);
    });

    expect(RecoveryService.prototype.verifyUser).toHaveBeenCalledWith({
      tokenId: startToken.id,
      username: startToken.username,
      password: 'password123',
    });

    expect(history.push).toHaveBeenLastCalledWith(
      route2NewDeviceWithApprovedToken
    );

    const deviceNameField = screen.getByPlaceholderText(/name/i);
    const registerKeyBtn = screen.getByText(/continue/i);

    fireEvent.change(deviceNameField, { target: { value: 'backup' } });
    fireEvent.click(registerKeyBtn);

    await wait(() => {
      fireEvent.click(screen.getByText(/continue/i));
      mockHistory.push(route3DevicesWithApprovedToken);
    });

    expect(RecoveryService.prototype.setNewU2fDevice).toHaveBeenCalledWith(
      expect.objectContaining({
        tokenId: approvedToken.id,
        deviceName: 'backup',
      })
    );

    expect(history.push).toHaveBeenLastCalledWith(
      route3DevicesWithApprovedToken
    );

    expect(MfaService.prototype.fetchDevicesWithToken).toHaveBeenCalledWith(
      approvedToken.id
    );

    expect(
      screen.getByText(/take a look at your enrolled devices below/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/iphone 12/i)).toBeInTheDocument();
    expect(screen.getByText(/solokey/i)).toBeInTheDocument();

    await wait(() => {
      fireEvent.click(screen.getByText('Continue'));
      mockHistory.push(routeNewCodesWithApprovedToken);
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
  });
});
