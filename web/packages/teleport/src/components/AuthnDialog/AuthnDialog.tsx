/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { ButtonIcon, ButtonSecondary, Flex, H2, Text } from 'design';
import { Danger } from 'design/Alert';
import Dialog, { DialogContent } from 'design/Dialog';
import { Cross, FingerprintSimple } from 'design/Icon';
import { guessProviderType } from 'shared/components/ButtonSso';
import { SSOIcon } from 'shared/components/ButtonSso/ButtonSso';

import { MfaCanceledError, MfaState } from 'teleport/lib/useMfa';
import { MFA_OPTION_TOTP } from 'teleport/services/mfa';

export type Props = {
  mfaState: MfaState;
  replaceErrorText?: string;
  // onClose is an optional function to perform additional operations
  // upon closing the dialog. e.g. close a shell session
  onClose?: () => void;
};

export default function AuthnDialog({
  mfaState: { options, challenge, submit, attempt, cancelAttempt },
  replaceErrorText,
  onClose = () => {},
}: Props) {
  const showError =
    attempt.status === 'error' && !(attempt.error instanceof MfaCanceledError);

  if (!challenge && !showError) return;

  // TODO(Joerger): TOTP should be pretty easy to support here with a small button -> form flow.
  const onlyTotpAvailable =
    options?.length === 1 && options[0] === MFA_OPTION_TOTP;

  return (
    <Dialog dialogCss={() => ({ width: '400px' })} open={true}>
      <Flex justifyContent="space-between" alignItems="center" mb={4}>
        <H2>Verify Your Identity</H2>
        <ButtonIcon
          data-testid="close-dialog"
          onClick={() => {
            cancelAttempt();
            onClose();
          }}
        >
          <Cross color="text.slightlyMuted" />
        </ButtonIcon>
      </Flex>
      <DialogContent mb={5}>
        {onlyTotpAvailable && (
          <Danger data-testid="danger-alert" mt={2} width="100%">
            {
              'Authenticator app is not currently supported for this action, please register a passkey or a security key to continue.'
            }
          </Danger>
        )}
        {attempt.status === 'error' && (
          <Danger data-testid="danger-alert" mt={2} width="100%">
            {replaceErrorText || attempt.statusText}
          </Danger>
        )}
        <Text color="text.slightlyMuted">
          {options?.length > 1
            ? 'Select one of the following methods to verify your identity:'
            : 'Select the method below to verify your identity:'}
        </Text>
      </DialogContent>
      {challenge && (
        <Flex textAlign="center" width="100%" flexDirection="column" gap={2}>
          {challenge.ssoChallenge && (
            <ButtonSecondary
              size="extra-large"
              onClick={() => submit('sso')}
              gap={2}
              block
            >
              <SSOIcon
                type={guessProviderType(
                  challenge.ssoChallenge.device.displayName ||
                    challenge.ssoChallenge.device.connectorId,
                  challenge.ssoChallenge.device.connectorType
                )}
              />
              {challenge.ssoChallenge.device.displayName ||
                challenge.ssoChallenge.device.connectorId}
            </ButtonSecondary>
          )}
          {challenge.webauthnPublicKey && (
            <ButtonSecondary
              size="extra-large"
              onClick={() => submit('webauthn')}
              gap={2}
              block
            >
              <FingerprintSimple />
              Passkey or MFA Device
            </ButtonSecondary>
          )}
        </Flex>
      )}
    </Dialog>
  );
}
