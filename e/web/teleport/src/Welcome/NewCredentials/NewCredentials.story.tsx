import { RecoveryCodes } from 'e-teleport/RecoveryCodes';
import { WelcomeWrapper } from 'teleport/components/Onboard';
import { NewCredentialsProps } from 'teleport/Welcome/NewCredentials';
import { NewCredentials } from 'teleport/Welcome/NewCredentials/NewCredentials';

export default {
  title: 'TeleportE/Welcome/NewCredentials',
  component: NewCredentials,
};

export const RecoveryCodesInvite = () => renderNewCredentials();

export const RecoveryCodesReset = () =>
  renderNewCredentials({
    resetMode: true,
  });

const makeNewCredProps = (
  overrides: Partial<NewCredentialsProps> = {}
): NewCredentialsProps => {
  return Object.assign(
    {
      auth2faType: 'off',
      primaryAuthType: 'local',
      isPasswordlessEnabled: true,
      submitAttempt: { status: '' },
      clearSubmitAttempt: () => null,
      fetchAttempt: { status: 'success' },
      onSubmitWithWebauthn: () => null,
      createNewWebAuthnDevice: () => null,
      onSubmit: () => null,
      redirect: () => null,
      success: false,
      finishedRegister: () => null,
      recoveryCodes: {
        codes: [
          'tele-testword-testword-testword-testword-testword-testword-testword',
          'tele-testword-testword-testword-testword-testword-testword-testword-testword',
          'tele-testword-testword-testword-testword-testword-testword-testword',
        ],
        createdDate: new Date('2019-08-30T11:00:00.00Z'),
      },
      resetToken: {
        user: 'john@example.com',
        tokenId: 'test123',
        qrCode: '',
      },
      isDashboard: false,
      RecoveryCodes,
    },
    overrides
  );
};

/**
 * Renders New Credentials
 *
 * @remarks
 * renderNewCredentials wraps the NewCredentials component in a WelcomeWrapper. Every instance of NewCredentials
 * is wrapped in a WelcomeWrapper via the Welcome parent component.
 *
 * @param partialProps - partial NewCredentialProps to override default values on individual stories
 */
const renderNewCredentials = (
  partialProps: Partial<NewCredentialsProps> = {}
) => {
  const props = makeNewCredProps(partialProps);

  return (
    <WelcomeWrapper>
      <NewCredentials {...props} />
    </WelcomeWrapper>
  );
};
