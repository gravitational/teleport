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
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
} from 'design/Dialog';
import { Danger } from 'design/Alert';
import { Text, ButtonSecondary, Flex, ButtonLink } from 'design';

import { guessProviderType } from 'shared/components/ButtonSso';
import { SSOIcon } from 'shared/components/ButtonSso/ButtonSso';

import { MfaState } from 'teleport/lib/useMfa';

export default function AuthnDialog({ mfa, onCancel }: Props) {
  return (
    <Dialog dialogCss={() => ({ width: '400px' })} open={true}>
      <DialogHeader style={{ flexDirection: 'column' }}>
        <DialogTitle textAlign="center">
          Multi-factor authentication
        </DialogTitle>
      </DialogHeader>
      <DialogContent mb={6}>
        {mfa.errorText && (
          <Danger data-testid="danger-alert" mt={2} width="100%">
            {mfa.errorText}
          </Danger>
        )}
        <Text textAlign="center">
          Re-enter your multi-factor authentication in the browser to continue.
        </Text>
      </DialogContent>
      <Flex textAlign="center" width="100%" flexDirection="column" gap={3}>
        {mfa.ssoChallenge && (
          <ButtonSecondary onClick={mfa.onSsoAuthenticate} gap={2} block>
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
          <ButtonSecondary onClick={mfa.onWebauthnAuthenticate} block>
            Passkey/Hardware Key
          </ButtonSecondary>
        )}
        <ButtonLink block onClick={onCancel}>
          Cancel
        </ButtonLink>
      </Flex>
    </Dialog>
  );
}

export type Props = {
  mfa: MfaState;
  onCancel: () => void;
};
