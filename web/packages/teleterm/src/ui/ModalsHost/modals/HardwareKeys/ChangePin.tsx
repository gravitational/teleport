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

import { ButtonPrimary, Flex, P2, Toggle } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import {
  PromptHardwareKeyPINChangeRequest,
  PromptHardwareKeyPINChangeResponse,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import {
  requiredAll,
  requiredField,
  Rule,
} from 'shared/components/Validation/rules';

import { CommonHeader } from './CommonHeader';

const DEFAULT_PIN = '123456';
const DEFAULT_PUK = '12345678';

export function ChangePin(props: {
  req: PromptHardwareKeyPINChangeRequest;
  onCancel(): void;
  onSuccess(response: PromptHardwareKeyPINChangeResponse): void;
  hidden?: boolean;
}) {
  const [pin, setPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const [puk, setPuk] = useState('');
  const [newPuk, setNewPuk] = useState('');
  const [confirmNewPuk, setConfirmNewPuk] = useState('');
  const [pukChanged, setPukChanged] = useState(false);

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
              validator.validate() &&
                props.onSuccess({
                  pin,
                  pukChanged: pukChanged,
                  puk: pukChanged ? newPuk : puk,
                });
            }}
          >
            <CommonHeader
              proxyHostname={props.req.proxyHostname}
              onCancel={props.onCancel}
            />

            <DialogContent mb={4}>
              <Flex flexDirection="column" gap={4} alignItems="flex-start">
                <P2>
                  The default PIV PIN is not allowed.
                  <br />
                  Please set a new PIV PIN for your hardware key before
                  proceeding. Both the PIN and PUK must be 6–8 characters long.
                </P2>

                <FieldInput
                  autoFocus
                  type="password"
                  label="New PIV PIN"
                  value={pin}
                  onChange={e => setPin(e.target.value)}
                  mb={0}
                  rule={requiredAll(
                    requiredField('New PIV PIN is required'),
                    notDefaultPin(),
                    correctLength('PIV PIN must be 6–8 characters long')
                  )}
                />
                <FieldInput
                  type="password"
                  label="Confirm New PIV PIN"
                  value={confirmPin}
                  onChange={e => setConfirmPin(e.target.value)}
                  mb={0}
                  rule={requiredAll(
                    requiredConfirmed(pin, {
                      confirm: 'Confirm New PIV PIN is required',
                      doesNotMatch: 'PIV PIN does not match',
                    }),
                    notDefaultPin(),
                    correctLength('PIV PIN must be 6–8 characters long')
                  )}
                />

                <P2>
                  To change the PIN, please provide your PUK code.
                  <br />
                  For security purposes, the default PUK ({DEFAULT_PUK}) cannot
                  be used. If you haven't changed your PUK yet, you'll need to
                  update it now.
                </P2>
                <Toggle
                  isToggled={pukChanged}
                  onToggle={() => setPukChanged(p => !p)}
                >
                  <Flex flexDirection="column" ml={2}>
                    Set up new PUK
                  </Flex>
                </Toggle>

                {!pukChanged ? (
                  <FieldInput
                    type="password"
                    label="PUK"
                    value={puk}
                    onChange={e => setPuk(e.target.value)}
                    mb={0}
                    rule={requiredAll(
                      requiredField('PUK is required'),
                      notDefaultPuk(),
                      correctLength('PUK must be 6–8 characters long')
                    )}
                  />
                ) : (
                  <>
                    <FieldInput
                      type="password"
                      label="New PUK"
                      value={newPuk}
                      onChange={e => setNewPuk(e.target.value)}
                      mb={0}
                      rule={requiredAll(
                        requiredField('New PUK is required'),
                        notDefaultPuk(),
                        correctLength('PUK must be 6–8 characters long')
                      )}
                    />
                    <FieldInput
                      type="password"
                      label="Confirm New PUK"
                      value={confirmNewPuk}
                      onChange={e => setConfirmNewPuk(e.target.value)}
                      mb={0}
                      rule={requiredAll(
                        requiredConfirmed(newPuk, {
                          confirm: 'Confirm New PUK is required',
                          doesNotMatch: 'PUK does not match',
                        }),
                        notDefaultPuk(),
                        correctLength('PUK must be 6–8 characters long')
                      )}
                    />
                  </>
                )}
              </Flex>
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

const notDefaultPin =
  <T = string,>(): Rule<T | T[] | readonly T[]> =>
  value =>
  () => {
    const message = 'Default PIN is not allowed';
    const valid = value !== DEFAULT_PIN;
    return {
      valid,
      message: !valid ? message : '',
    };
  };

const notDefaultPuk =
  <T = string,>(): Rule<T | T[] | readonly T[]> =>
  value =>
  () => {
    const message = 'Default PUK is not allowed';
    const valid = value !== DEFAULT_PUK;
    return {
      valid,
      message: !valid ? message : '',
    };
  };

const correctLength =
  <T = string,>(message: string): Rule<T | T[] | readonly T[]> =>
  value =>
  () => {
    const valid =
      typeof value === 'string' && value.length >= 6 && value.length <= 8;
    return {
      valid,
      message: !valid ? message : '',
    };
  };

const requiredConfirmed =
  (
    code: string,
    messages: {
      confirm: string;
      doesNotMatch: string;
    }
  ): Rule =>
  (confirmedCode: string) =>
  () => {
    if (!confirmedCode) {
      return {
        valid: false,
        message: messages.confirm,
      };
    }

    if (confirmedCode !== code) {
      return {
        valid: false,
        message: messages.doesNotMatch,
      };
    }

    return {
      valid: true,
    };
  };
