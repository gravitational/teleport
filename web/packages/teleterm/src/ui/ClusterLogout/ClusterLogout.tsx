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
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import * as Alerts from 'design/Alert';
import { ButtonIcon, ButtonPrimary, Text } from 'design';
import { Props, State, useClusterLogout } from './useClusterLogout';
import { Close } from 'design/Icon';

export default function Container(props: Props) {
  const state = useClusterLogout(props);
  return <ClusterLogout {...state} />;
}

export function ClusterLogout({
  status,
  onClose,
  statusText,
  clusterTitle,
  removeCluster,
}: State) {
  return (
    <DialogConfirmation
      open={true}
      onClose={onClose}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          removeCluster();
        }}
      >
        <DialogHeader justifyContent="space-between" mb={0}>
          <Text typography="h5" bold style={{ whiteSpace: 'nowrap' }}>
            Log out from cluster {clusterTitle}
          </Text>
          <ButtonIcon
            type="button"
            disabled={status === 'processing'}
            onClick={onClose}
            color="text.secondary"
          >
            <Close fontSize={5} />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={4}>
          <Text color="text.secondary" typography="body1">
            Are you sure you want to log out?
          </Text>
          {status === 'error' && <Alerts.Danger mb={5} children={statusText} />}
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary
            kind="warning"
            disabled={status === 'processing'}
            size="large"
            block={true}
            autoFocus
            type="submit"
          >
            Log out
          </ButtonPrimary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
