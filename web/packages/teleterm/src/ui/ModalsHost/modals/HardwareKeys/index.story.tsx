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

export default {
  title: 'Teleterm/ModalsHost/HardwareKeys',
} satisfies Meta;

export function AskPinOptional() {
  return (
    <AskPinComponent
      onSuccess={() => {}}
      onCancel={() => {}}
      req={{
        rootClusterUri: rootCluster.uri,
        pinOptional: true,
      }}
    />
  );
}

export function AskPinRequired() {
  return (
    <AskPinComponent
      onSuccess={() => {}}
      onCancel={() => {}}
      req={{
        rootClusterUri: rootCluster.uri,
        pinOptional: false,
      }}
    />
  );
}

export function Touch() {
  return (
    <TouchComponent
      onCancel={() => {}}
      req={{
        rootClusterUri: rootCluster.uri,
      }}
    />
  );
}

export function ChangePin() {
  return (
    <ChangePinComponent
      onSuccess={() => {}}
      onCancel={() => {}}
      req={{ rootClusterUri: rootCluster.uri }}
    />
  );
}

export function OverwriteSlot() {
  return (
    <OverwriteSlotComponent
      onConfirm={() => {}}
      onCancel={() => {}}
      req={{
        rootClusterUri: rootCluster.uri,
        message:
          "Would you like to overwrite this slot's private key and certificate?",
      }}
    />
  );
}
