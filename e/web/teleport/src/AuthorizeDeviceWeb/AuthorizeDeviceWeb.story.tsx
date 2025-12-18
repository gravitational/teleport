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
import { MemoryRouter } from 'react-router';

import { Platform } from 'design/platform';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { getConnectDownloadLinks } from 'teleport/components/DownloadConnect/DownloadConnect';
import { ContentMinWidth } from 'teleport/Main/Main';

import { DeviceTrustConnectPassthrough } from './AuthorizeDeviceWeb';

type StoryProps = {
  platform: Platform;
};

const meta: Meta<StoryProps> = {
  title: 'TeleportE/AuthorizeDeviceWeb',
  component: AuthorizeDeviceWeb,
  argTypes: {
    platform: {
      control: { type: 'select' },
      options: Object.values(Platform),
    },
  },
  args: {
    platform: Platform.macOS,
  },
};
export default meta;

export function AuthorizeDeviceWeb(props: StoryProps) {
  const downloadLinks = getConnectDownloadLinks(props.platform, '15.2.2');
  return (
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContentMinWidth>
          <DeviceTrustConnectPassthrough
            authorizeWebDeviceDeepLink={'blank'}
            downloadLinks={downloadLinks}
          />
        </ContentMinWidth>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );
}
