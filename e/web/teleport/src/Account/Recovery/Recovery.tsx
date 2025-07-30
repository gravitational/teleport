import { ButtonPrimary, Text } from 'design';
import * as Icon from 'design/Icon';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import { P } from 'design/Text/Text';

import useTeleportE from 'e-teleport/useTeleportE';
import { EnterpriseComponentProps } from 'teleport/Account/Account';
import { Header } from 'teleport/Account/Header';
import ReAuthenticate from 'teleport/components/ReAuthenticate';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';

import RecoveryCodesDialog from './RecoveryCodesDialog';
import useRecovery, { State } from './useRecovery';

export default function Container({
  addNotification,
}: EnterpriseComponentProps) {
  const ctx = useTeleportE();
  const state = useRecovery(ctx, msg =>
    addNotification({ severity: 'error', content: msg })
  );
  return <Recovery {...state} />;
}

export function Recovery({
  attempt,
  token,
  setToken,
  showReAuthenticate,
  hideReAuthenticate,
  isReAuthenticateVisible,
  hideCodes,
  isCodesVisible,
  fetchCreatedDate,
  userHasCodes,
  isRecoveryEnabled,
  createdDateText,
}: State) {
  const title = userHasCodes
    ? 'Generate New Recovery Codes'
    : 'Generate Recovery Codes';

  const description = () => {
    if (!isRecoveryEnabled)
      return 'Account recovery is only available for local users with a valid email as their username.';
    if (userHasCodes)
      return 'When you generate new recovery codes, your old ones will no longer work. Please make sure to save the new codes in a safe offline place. You can use each code once if you lose your second factor authenticator or password.';
    return 'Recovery codes are one-time use passcodes. You can use each code once if you lose your second factor authenticator or password. You haven’t generated any codes yet. Please generate them now and store them in a safe offline place.';
  };

  const buttonText = userHasCodes
    ? 'Generate new recovery codes'
    : 'Generate recovery codes';

  return (
    <>
      <MultiRowBox>
        <Row>
          <Header
            icon={<Icon.Vault />}
            title={title}
            description={description()}
            showIndicator={attempt.status === 'processing'}
            actions={
              isRecoveryEnabled && (
                <ButtonPrimary size="large" onClick={showReAuthenticate}>
                  {buttonText}
                </ButtonPrimary>
              )
            }
          />
        </Row>
        {isRecoveryEnabled && userHasCodes && (
          <Row>
            <P fontSize={3}>
              Recovery codes were last generated on:{' '}
              <Text as="span" bold>
                {createdDateText}
              </Text>
            </P>
          </Row>
        )}
      </MultiRowBox>
      {isReAuthenticateVisible && (
        <ReAuthenticate
          challengeScope={MfaChallengeScope.MANAGE_DEVICES}
          onMfaResponse={mfaResponse =>
            auth.createPrivilegeToken(mfaResponse).then(setToken)
          }
          onClose={hideReAuthenticate}
        />
      )}
      {isCodesVisible && (
        <RecoveryCodesDialog
          token={token}
          close={hideCodes}
          refreshDate={fetchCreatedDate}
          isNewCodes={userHasCodes}
        />
      )}
    </>
  );
}
