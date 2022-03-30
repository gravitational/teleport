/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import * as Alerts from 'design/Alert';
import { Text, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogHeader,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import useGatewayCreate, { State, Props } from './useGatewayCreate';

export default function Container(props: Props) {
  const state = useGatewayCreate(props);
  return <GatewayCreate {...state} />;
}

export function GatewayCreate(props: State) {
  const {
    db,
    port,
    user,
    userRequired,
    setUser,
    setPort,
    onClose,
    createAttempt,
    createGateway,
  } = props;

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={() => ({
            maxWidth: '400px',
            width: '100%',
            padding: '20px',
          })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <form
            onSubmit={e => {
              e.preventDefault();
              validator.validate() && createGateway({ port, user });
            }}
          >
            <DialogHeader>
              <Text typography="h4" color="text.primary">
                Create connection <b>{db.name}</b>
              </Text>
            </DialogHeader>
            <DialogContent>
              {createAttempt.status === 'error' && (
                <Alerts.Danger>{createAttempt.statusText}</Alerts.Danger>
              )}
              {userRequired && (
                <FieldInput
                  label="User"
                  width="100%"
                  value={user}
                  autoFocus
                  placeholder="mongo db user"
                  onChange={e => setUser(e.target.value)}
                />
              )}
              <FieldInput
                label="Port (Optional)"
                width="100%"
                value={port}
                autoFocus={!userRequired}
                placeholder="10211"
                onChange={e => setPort(e.target.value)}
              />
            </DialogContent>
            <DialogFooter>
              <ButtonPrimary
                mr="3"
                disabled={createAttempt.status === 'processing'}
                type="submit"
              >
                Create
              </ButtonPrimary>
              <ButtonSecondary type="button" onClick={onClose}>
                Close
              </ButtonSecondary>
            </DialogFooter>
          </form>
        </Dialog>
      )}
    </Validation>
  );
}
