/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { UpdateInfo } from 'electron-updater';

import Flex from 'design/Flex';

import { AppUpdateEvent } from 'teleterm/services/appUpdater';
import { Details } from 'teleterm/ui/AppUpdater/Details';
import { Widget } from 'teleterm/ui/AppUpdater/Widget';

const update: UpdateInfo = {
  files: [{ url: '', sha512: '', size: 123214312 }],
  releaseDate: new Date().toDateString(),
  version: '18.1.0',
  path: '',
  sha512: '',
};

const updateNotAvailable: AppUpdateEvent = {
  kind: 'update-not-available',
};

const checkingForUpdate: AppUpdateEvent = {
  kind: 'checking-for-update',
};
const updateAvailable: AppUpdateEvent = {
  kind: 'update-available',
  update: update,
};
const downloadProgress: AppUpdateEvent = {
  kind: 'download-progress',
  progress: {
    total: 123214312,
    transferred: 4322432,
    percent: 12,
    delta: 1,
    bytesPerSecond: 12333,
  },
  update: update,
};
const error: AppUpdateEvent = {
  kind: 'error',
  error: new Error('No permissions'),
  update: update,
};
const updateDownloaded: AppUpdateEvent = {
  kind: 'update-downloaded',
  update: update,
};
const allEvents: AppUpdateEvent[] = [
  updateNotAvailable,
  checkingForUpdate,
  updateAvailable,
  downloadProgress,
  error,
  updateDownloaded,
];

const meta: Meta<StoryProps> = {
  title: 'Teleterm/AppUpdate',
  argTypes: {
    state: {
      control: { type: 'radio' },
      options: allEvents.map(({ kind }) => kind),
      description: 'Update state',
    },
  },
  args: {
    state: 'update-available',
  },
};
export default meta;

export interface StoryProps {
  state: AppUpdateEvent['kind'];
}

export const ExpandedView = (storyProps: StoryProps) => {
  const event = allEvents.find(event => storyProps.state === event.kind);
  return (
    <Flex maxWidth="500px">
      <Details
        onDownload={() => {}}
        onInstall={() => {}}
        onCheckForUpdates={() => {}}
        managedInfo={{ cluster: '', version: '18.0.0' }}
        updateEvent={event}
      />
    </Flex>
  );
};
export const WidgetView = (storyProps: StoryProps) => {
  const event = allEvents.find(event => storyProps.state === event.kind);
  return (
    <Flex maxWidth="440px">
      <Widget update={event} onChangeFlow={() => {}} />
    </Flex>
  );
};
