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

import React from 'react';
import Dialog, { DialogContent } from 'design/Dialog';
import { Danger } from 'design/Alert';
import { FingerprintSimple, Cross } from 'design/Icon';

import { Text, ButtonSecondary, Flex, ButtonIcon, H2 } from 'design';

import { guessProviderType } from 'shared/components/ButtonSso';
import { SSOIcon } from 'shared/components/ButtonSso/ButtonSso';

import { MfaState } from 'teleport/lib/useMfa';

export default function AuthnDialog({ mfa, onCancel }: Props) {
  return (
    <Dialog dialogCss={() => ({ width: '400px' })} open={true}>
      <Flex justifyContent="space-between" alignItems="center" mb={4}>
        <H2>Re-authenticate in the Browser</H2>
        <ButtonIcon data-testid="close-dialog" onClick={onCancel}>
          <Cross color="text.slightlyMuted" />
        </ButtonIcon>
      </Flex>
      <DialogContent mb={5}>
        {mfa.errorText && (
          <Danger data-testid="danger-alert" mt={2} width="100%">
            {mfa.errorText}
          </Danger>
        )}
        <Text textAlign="center" color="text.slightlyMuted">
          To continue, you must verify your identity by re-authenticating:
        </Text>
      </DialogContent>
      <Flex textAlign="center" width="100%" flexDirection="column" gap={2}>
        {mfa.ssoChallenge && (
          <ButtonSecondary
            size="extra-large"
            onClick={mfa.onSsoAuthenticate}
            gap={2}
            block
          >
            <SSOIcon
              type={guessProviderType(
                mfa.ssoChallenge.device.displayName,
                mfa.ssoChallenge.device.connectorType
              )}
            />
            {mfa.ssoChallenge.device.displayName}
          </ButtonSecondary>
        )}
        {mfa.webauthnPublicKey && (
          <ButtonSecondary
            size="extra-large"
            onClick={mfa.onWebauthnAuthenticate}
            gap={2}
            block
          >
            <FingerprintSimple />
            Passkey or MFA Device
          </ButtonSecondary>
        )}
      </Flex>
    </Dialog>
  );
}

export type Props = {
  mfa: MfaState;
  onCancel: () => void;
};
