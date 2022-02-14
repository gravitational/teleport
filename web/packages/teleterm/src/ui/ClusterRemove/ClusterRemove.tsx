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
import { Text, ButtonPrimary, ButtonSecondary } from 'design';
import { State, Props, useClusterRemove } from './useClusterRemove';

export default function Container(props: Props) {
  const state = useClusterRemove(props);
  return <ClusterRemove {...state} />;
}

export function ClusterRemove({
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
        maxWidth: '380px',
        width: '100%',
      })}
    >
      <DialogHeader>
        <Text typography="h4" css={{ whiteSpace: 'nowrap' }}>
          Remove Cluster {clusterTitle}
        </Text>
      </DialogHeader>
      <DialogContent>
        <Text color="text.secondary" typography="h5">
          Are you sure you want to remove cluster?
        </Text>
        {status === 'error' && <Alerts.Danger mb={5} children={statusText} />}
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          disabled={status === 'processing'}
          mr="3"
          onClick={e => {
            e.preventDefault();
            removeCluster();
          }}
        >
          Remove
        </ButtonPrimary>
        <ButtonSecondary
          disabled={status === 'processing'}
          onClick={e => {
            e.preventDefault();
            onClose();
          }}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </DialogConfirmation>
  );
}
