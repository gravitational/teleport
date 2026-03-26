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

import { Meta } from '@storybook/react-vite';
import { useState } from 'react';

import {
  makeEmptyAttempt,
  makeErrorAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';

import { routing } from 'teleterm/ui/uri';

import { HeadlessPrompt } from './HeadlessPrompt';

type StoryProps = {
  clusterName: string;
  approve: 'succeed' | 'throw-error' | 'processing';
  reject: 'succeed' | 'throw-error' | 'processing';
};

const meta: Meta<StoryProps> = {
  title: 'Teleterm/ModalsHost/HeadlessPrompt',
  argTypes: {
    approve: {
      control: { type: 'radio' },
      options: ['succeed', 'throw-error', 'processing'],
    },
    reject: {
      control: { type: 'radio' },
      options: ['succeed', 'throw-error', 'processing'],
    },
  },
  args: {
    clusterName: 'teleport-local',
    approve: 'succeed',
    reject: 'succeed',
  },
};
export default meta;

export const Story = (props: StoryProps) => {
  const [attempt, setAttempt] = useState(makeEmptyAttempt<void>());
  const [key, setKey] = useState(crypto.randomUUID());

  return (
    <HeadlessPrompt
      key={key}
      rootClusterUri={routing.ensureRootClusterUri(
        `/clusters/${props.clusterName}`
      )}
      clientIp="localhost"
      skipConfirm={false}
      onApprove={async () => {
        if (props.approve === 'succeed') {
          setAttempt(makeSuccessAttempt(undefined));
        } else if (props.approve === 'throw-error') {
          setAttempt(makeErrorAttempt(error));
        }
      }}
      abortApproval={() => {}}
      onReject={async () => {
        if (props.reject === 'succeed') {
          setAttempt(makeSuccessAttempt(undefined));
        } else if (props.reject === 'throw-error') {
          setAttempt(makeErrorAttempt(error));
        }
      }}
      updateHeadlessStateAttempt={attempt}
      onCancel={() => {
        setAttempt(makeEmptyAttempt());
        setKey(crypto.randomUUID());
      }}
      headlessAuthenticationId="85fa45fa-57f4-5a9d-9ba8-b3cbf76d5ea2"
    />
  );
};

const error = new Error(
  'failed to authenticate using available MFA devices\n    Webauthn authentication failed\n    failed to get assertion: rx invalid cbor, rpc error: code = Aborted desc = MFA modal closed by user'
);
