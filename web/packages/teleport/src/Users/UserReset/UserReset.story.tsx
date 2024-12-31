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

import { Meta } from '@storybook/react';
import { useEffect, useState } from 'react';

import { Attempt } from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';

import { UserReset } from './UserReset';

type StoryProps = {
  status: 'processing' | 'success' | 'error';
  isMfaEnabled: boolean;
  allowPasswordless: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Users/UserReset',
  component: Story,
  argTypes: {
    status: {
      control: { type: 'radio' },
      options: ['processing', 'success', 'error'],
    },
  },
  args: {
    status: 'processing',
    isMfaEnabled: true,
    allowPasswordless: true,
  },
};

export default meta;

export function Story(props: StoryProps) {
  const statusToAttempt: Record<StoryProps['status'], Attempt> = {
    processing: { status: 'processing' },
    success: { status: 'success' },
    error: { status: 'failed', statusText: 'some server error' },
  };
  const [, setState] = useState({});

  useEffect(() => {
    const defaultAuth = structuredClone(cfg.auth);
    cfg.auth.second_factor = props.isMfaEnabled ? 'on' : 'off';
    cfg.auth.allowPasswordless = props.allowPasswordless;
    setState({}); // Force re-render of the component with new cfg.

    return () => {
      cfg.auth = defaultAuth;
    };
  }, [props.isMfaEnabled, props.allowPasswordless]);

  return (
    <UserReset
      username="smith"
      token={{
        value: '0c536179038b386728dfee6602ca297f',
        expires: new Date('2021-04-08T07:30:00Z'),
        username: 'Lester',
      }}
      onReset={() => {}}
      onClose={() => {}}
      attempt={statusToAttempt[props.status]}
    />
  );
}
