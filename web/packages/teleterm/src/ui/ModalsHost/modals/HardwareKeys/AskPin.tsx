/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useState } from 'react';

import { ButtonPrimary, P2, Stack } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import { PromptHardwareKeyPINRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { CliCommand } from 'teleterm/ui/components/CliCommand';

import { CommonHeader } from './CommonHeader';

export function AskPin(props: {
  req: PromptHardwareKeyPINRequest;
  onCancel(): void;
  onSuccess(pin: string): void;
  hidden?: boolean;
}) {
  const [pin, setPin] = useState('');

  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '450px',
        width: '100%',
      })}
    >
      <Validation>
        {({ validator }) => (
          <form
            onSubmit={e => {
              e.preventDefault();
              validator.validate() && props.onSuccess(pin);
            }}
          >
            <CommonHeader
              onCancel={props.onCancel}
              proxyHostname={props.req.proxyHostname}
            />

            <DialogContent mb={4}>
              <Stack>
                <P2>
                  Enter your YubiKey PIV PIN to continue
                  {props.req.command ? ' with command:' : '.'}
                </P2>
                {props.req.command && (
                  <CliCommand cliCommand={props.req.command} wrapContent />
                )}
                <FieldInput
                  mt={3}
                  flex="1"
                  autoFocus
                  type="password"
                  rule={
                    props.req.pinOptional
                      ? undefined
                      : requiredField('PIV PIN is required')
                  }
                  label="PIV PIN"
                  value={pin}
                  onChange={e => setPin(e.target.value)}
                  placeholder="123 456"
                  mb={0}
                />
                <P2 color="text.slightlyMuted">
                  {props.req.pinOptional &&
                    'To change the default PIN, leave the field blank.'}
                </P2>
              </Stack>
            </DialogContent>

            <DialogFooter>
              <ButtonPrimary type="submit">Continue</ButtonPrimary>
            </DialogFooter>
          </form>
        )}
      </Validation>
    </DialogConfirmation>
  );
}
