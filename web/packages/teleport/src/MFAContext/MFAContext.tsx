import { PropsWithChildren, createContext, useContext, useState } from 'react';
import {
  Flex,
  Box,
  Text,
  ButtonPrimary,
  ButtonSecondary,
  H2,
  ButtonIcon,
} from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { Danger } from 'design/Alert';
import Validation from 'shared/components/Validation';
import { requiredToken } from 'shared/components/Validation/rules';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';

import createMfaOptions, { MfaOption } from 'shared/utils/createMfaOptions';

import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import {
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
} from 'teleport/services/mfa';
import useAttempt from 'shared/hooks/useAttemptNext';
import { isAdminActionRequiresMfaError } from 'teleport/services/api/api';
import parseError, { ApiError } from 'teleport/services/api/parseError';
import { guessProviderType } from 'shared/components/ButtonSso';
import { Cross, FingerprintSimple } from 'design/Icon';
import { Auth2faType } from 'shared/services';
import { SSOIcon } from 'shared/components/ButtonSso/ButtonSso';

export interface MFAContextValue {
  withMfaRetry(fn: (mfaResponse: MfaChallengeResponse) => any);
}

export const MFAContext = createContext<MFAContextValue>(null);

export const useMfaContext = () => {
  return useContext(MFAContext);
};

export const MFAContextProvider = ({ children }: PropsWithChildren) => {
  const { attempt, setAttempt } = useAttempt('');

  const [mfaChallenge, setMfaChallenge] = useState<MfaAuthenticateChallenge>();
  const [mfaResponsePromise, setMfaResponsePromise] =
    useState<PromiseWithResolvers<MfaChallengeResponse>>();

  const mfaOptions = createMfaOptions(mfaChallenge);
  const [mfaOption, setMfaOption] = useState<Auth2faType>();

  async function withMfaRetry(
    callback: (mfaResp?: MfaChallengeResponse) => any
  ): Promise<any> {
    // const response = await callback();

    // let json;
    // try {
    //   json = await response.json();
    // } catch (err) {
    //   const message = response.ok
    //     ? err.message
    //     : `${response.status} - ${response.url}`;
    //   throw new ApiError(message, response, { cause: err });
    // }

    // if (response.ok) {
    //   return json;
    // }

    // // Retry with MFA if we get an admin action missing MFA error.
    // const isAdminActionMfaError = isAdminActionRequiresMfaError(
    //   parseError(json)
    // );

    // if (!isAdminActionMfaError) {
    //   throw new ApiError(parseError(json), response, undefined, json.messages);
    // }

    setAttempt({ status: 'processing' });
    const challenge = await auth.getMfaChallenge({
      scope: MfaChallengeScope.ADMIN_ACTION,
    });
    setMfaChallenge(challenge);

    const promise = Promise.withResolvers();
    setMfaResponsePromise(promise);
    const mfaResponse = await promise.promise;

    console.log(mfaResponse);
    return callback(mfaResponse);
  }

  function onSubmit() {
    auth.getMfaChallengeResponse(mfaChallenge, mfaOption).then(resp => {
      mfaResponsePromise.resolve(resp);
    });
  }

  function clearAttempt() {
    setAttempt({ status: '' });
  }

  function onClose() {
    return;
  }

  return (
    <MFAContext.Provider value={{ withMfaRetry }}>
      {mfaChallenge && (
        <Dialog dialogCss={() => ({ width: '400px' })} open={true}>
          <Flex justifyContent="space-between" alignItems="center" mb={4}>
            <H2>Verify Your Identity</H2>
            <ButtonIcon data-testid="close-dialog" onClick={onClose}>
              <Cross color="text.slightlyMuted" />
            </ButtonIcon>
          </Flex>
          <DialogContent mb={5}>
            {attempt.statusText && (
              <Danger data-testid="danger-alert" mt={2} width="100%">
                {attempt.statusText}
              </Danger>
            )}
            <Text color="text.slightlyMuted">
              {mfaOptions.length > 0
                ? 'Select one of the following methods to verify your identity:'
                : 'Select the method below to verify your identity:'}
            </Text>
          </DialogContent>
          <Flex textAlign="center" width="100%" flexDirection="column" gap={2}>
            {mfaChallenge.ssoChallenge && (
              <ButtonSecondary
                size="extra-large"
                onClick={() => {
                  setMfaOption('sso');
                  onSubmit();
                }}
                gap={2}
                block
              >
                <SSOIcon
                  type={guessProviderType(
                    mfaChallenge.ssoChallenge.device.displayName ||
                      mfaChallenge.ssoChallenge.device.connectorId,
                    mfaChallenge.ssoChallenge.device.connectorType
                  )}
                />
                {mfaChallenge.ssoChallenge.device.displayName ||
                  mfaChallenge.ssoChallenge.device.connectorId}
              </ButtonSecondary>
            )}
            {mfaChallenge.webauthnPublicKey && (
              <ButtonSecondary
                size="extra-large"
                onClick={() => {
                  setMfaOption('webauthn');
                  onSubmit();
                }}
                gap={2}
                block
              >
                <FingerprintSimple />
                Passkey or MFA Device
              </ButtonSecondary>
            )}
          </Flex>
        </Dialog>
      )}
      {children}
    </MFAContext.Provider>
  );
};
