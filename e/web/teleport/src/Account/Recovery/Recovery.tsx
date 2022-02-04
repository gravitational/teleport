import React from 'react';
import { Text, Box, ButtonPrimary, Indicator } from 'design';
import { Danger } from 'design/Alert';
import ReAuthenticate from 'teleport/components/ReAuthenticate';
import RecoveryCodesDialog from './RecoveryCodesDialog';
import useRecovery, { State } from './useRecovery';
import useTeleportE from 'e-teleport/useTeleportE';

export default function Container() {
  const ctx = useTeleportE();
  const state = useRecovery(ctx);
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

  const description = userHasCodes
    ? 'When you generate new recovery codes, your old ones will no longer work. Please make sure to save the new codes in a safe offline place. You can use each code once if you lose your second factor authenticator or password.'
    : 'Recovery codes are one-time use passcodes. You can use each code once if you lose your second factor authenticator or password. You haven’t generated any codes yet. Please generate them now and store them in a safe offline place.';

  const buttonText = userHasCodes
    ? 'Generate new recovery codes'
    : 'Generate recovery codes';

  return (
    <>
      {attempt.status === 'failed' && (
        <Danger width="900px">{attempt.statusText}</Danger>
      )}
      {attempt.status === 'processing' && (
        <Box width="900px" textAlign="center">
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <>
          <Box width="900px" px={4} py={4} bg="primary.light" borderRadius={3}>
            {isRecoveryEnabled ? (
              <>
                <Text typography="h4" bold mb={3}>
                  {title}
                </Text>
                <Text typography="paragraph" mb={5}>
                  {description}
                </Text>
                {userHasCodes && (
                  <Text typography="body1" fontSize={3} mb={6}>
                    Recovery codes were last generated on:{' '}
                    <Text as="span" bold>
                      {createdDateText}
                    </Text>
                  </Text>
                )}
                <ButtonPrimary size="large" onClick={showReAuthenticate}>
                  {buttonText}
                </ButtonPrimary>{' '}
              </>
            ) : (
              <Text typography="paragraph" textAlign="center">
                Account recovery is only available for local users with a valid
                email as their username.
              </Text>
            )}
          </Box>
          {isReAuthenticateVisible && (
            <ReAuthenticate
              onAuthenticated={setToken}
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
      )}
    </>
  );
}
