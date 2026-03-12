import { MemoryRouter } from 'react-router';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import cfg from 'e-teleport/config';
import RecoveryService from 'e-teleport/services/recovery';
import history from 'teleport/services/history';
import MfaService from 'teleport/services/mfa';

import { Recovery } from '../Recovery';

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
  // Helper component that provides routing context
  const TestWrapper = ({
    initialPath,
    children,
  }: {
    initialPath: string;
    children: React.ReactNode;
  }) => {
    return (
      <MemoryRouter key={initialPath} initialEntries={[initialPath]}>
        {children}
      </MemoryRouter>
    );
  };

  const setup = (initialPath: string = '/web/recovery/startTokenId') => {
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
          type: 'totp',
          usage: 'mfa',
        },
        {
          id: '2',
          description: 'Hardware Key',
          name: 'solokey',
          registeredDate: new Date(1623722252),
          lastUsedDate: new Date(1623981452),
          type: 'webauthn',
          usage: 'mfa',
        },
      ]);

    const { rerender } = render(
      <TestWrapper initialPath={initialPath}>
        <Recovery />
      </TestWrapper>
    );

    // Return a function to re-render with a new path
    const rerenderWithPath = (newPath: string) => {
      rerender(
        <TestWrapper initialPath={newPath}>
          <Recovery />
        </TestWrapper>
      );
    };

    return { rerenderWithPath };
  };

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('new password using otp', async () => {
    const { rerenderWithPath } = setup();

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

    rerenderWithPath(route1VerifyWithStartToken);

    await waitFor(() => {
      expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);
    });

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: true,
      });

    const tokenField = await screen.findByPlaceholderText('123 456');

    fireEvent.change(tokenField, { target: { value: '321321' } });

    fireEvent.click(screen.getByText('Continue'));
    rerenderWithPath(route2NewPasswordWithApprovedToken);
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

    const newPasswordField = await screen.findByPlaceholderText('Password');
    const newConfirmPasswordField =
      screen.getByPlaceholderText('Confirm Password');

    fireEvent.change(newPasswordField, { target: { value: 'password1234' } });
    fireEvent.change(newConfirmPasswordField, {
      target: { value: 'password1234' },
    });

    fireEvent.click(screen.getByText('Continue'));
    rerenderWithPath(routeNewCodesWithApprovedToken);

    await waitFor(() => {
      expect(
        RecoveryService.prototype.setNewTotpDeviceOrPassword
      ).toHaveBeenCalledWith({
        tokenId: approvedToken.id,
        password: 'password1234',
      });
    });

    expect(history.push).toHaveBeenLastCalledWith(
      routeNewCodesWithApprovedToken
    );

    await waitFor(() => {
      expect(
        RecoveryService.prototype.generateRecoveryCodes
      ).toHaveBeenCalledWith(approvedToken.id);
    });

    expect(
      await screen.findByText('New Backup & Recovery Codes')
    ).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
    expect(
      screen.getByText(recoveryCodes.createdDate.toString(), { exact: false })
    ).toBeInTheDocument();
  });

  test('new otp device using password', async () => {
    const { rerenderWithPath } = setup();

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

    rerenderWithPath(route1VerifyWithStartToken);

    await waitFor(() => {
      expect(history.replace).toHaveBeenCalledWith(route1VerifyWithStartToken);
    });

    jest
      .spyOn(RecoveryService.prototype, 'fetchRecoveryToken')
      .mockResolvedValue({
        ...approvedToken,
        isRecoverPassword: false,
      });

    const passwordField = await screen.findByPlaceholderText('Password');

    fireEvent.change(passwordField, { target: { value: 'password1234' } });

    fireEvent.click(screen.getByText('Continue'));
    rerenderWithPath(route2NewDeviceWithApprovedToken);

    expect(RecoveryService.prototype.verifyUser).toHaveBeenCalledWith({
      tokenId: startToken.id,
      username: startToken.username,
      password: 'password1234',
    });

    await waitFor(() => {
      expect(history.push).toHaveBeenLastCalledWith(
        route2NewDeviceWithApprovedToken
      );
    });

    const newTokenField = await screen.findByPlaceholderText('123 456');
    const deviceNameField = screen.getByPlaceholderText(/name/i);

    fireEvent.change(newTokenField, { target: { value: '321321' } });
    fireEvent.change(deviceNameField, { target: { value: 'backup' } });

    fireEvent.click(screen.getByText('Continue'));
    rerenderWithPath(route3DevicesWithApprovedToken);

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
      await screen.findByText(/take a look at your enrolled devices below/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/iphone 12/i)).toBeInTheDocument();
    expect(screen.getByText(/solokey/i)).toBeInTheDocument();

    fireEvent.click(screen.getByText('Continue'));
    rerenderWithPath(routeNewCodesWithApprovedToken);

    expect(history.push).toHaveBeenLastCalledWith(
      routeNewCodesWithApprovedToken
    );

    await waitFor(() => {
      expect(
        RecoveryService.prototype.generateRecoveryCodes
      ).toHaveBeenCalledWith(approvedToken.id);
    });

    expect(
      await screen.findByText('New Backup & Recovery Codes')
    ).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-1/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-2/i)).toBeInTheDocument();
    expect(screen.getByText(/tele-recovery-code-3/i)).toBeInTheDocument();
    expect(
      screen.getByText(recoveryCodes.createdDate.toString(), { exact: false })
    ).toBeInTheDocument();
  });
});
