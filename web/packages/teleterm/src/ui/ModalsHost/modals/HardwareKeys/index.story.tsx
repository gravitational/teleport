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

import { Meta } from '@storybook/react';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { AskPin as AskPinComponent } from './AskPin';
import { ChangePin as ChangePinComponent } from './ChangePin';
import { OverwriteSlot as OverwriteSlotComponent } from './OverwriteSlot';
import { Touch as TouchComponent } from './Touch';

const rootCluster = makeRootCluster();

interface StoryProps {
  command: boolean;
}

const meta: Meta<StoryProps> = {
  title: 'Teleterm/ModalsHost/HardwareKeys',
  argTypes: {
    command: {
      control: { type: 'boolean' },
      description: 'Show a command when asked for pin or touch.',
    },
  },
  args: {
    command: true,
  },
};

export default meta;

const longCommand =
  'tsh ssh -X --forward-agent=yes --proxy=root.example.com --user=testuser';

export function AskPinOptional(props: StoryProps) {
  return (
    <AskPinComponent
      onSuccess={() => {}}
      onCancel={() => {}}
      req={{
        proxyHostname: rootCluster.proxyHost,
        pinOptional: true,
        command: props.command ? longCommand : '',
      }}
    />
  );
}

export function AskPinRequired(props: StoryProps) {
  return (
    <AskPinComponent
      onSuccess={() => {}}
      onCancel={() => {}}
      req={{
        proxyHostname: rootCluster.proxyHost,
        pinOptional: false,
        command: props.command ? longCommand : '',
      }}
    />
  );
}

export function Touch(props: StoryProps) {
  return (
    <TouchComponent
      onCancel={() => {}}
      req={{
        proxyHostname: rootCluster.proxyHost,
        command: props.command ? longCommand : '',
      }}
    />
  );
}

export function ChangePin() {
  return (
    <ChangePinComponent
      onSuccess={() => {}}
      onCancel={() => {}}
      req={{
        proxyHostname: rootCluster.proxyHost,
      }}
    />
  );
}

export function OverwriteSlot() {
  return (
    <OverwriteSlotComponent
      onConfirm={() => {}}
      onCancel={() => {}}
      req={{
        proxyHostname: rootCluster.proxyHost,
        message:
          "Would you like to overwrite this slot's private key and certificate?",
      }}
    />
  );
}
