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

import { ButtonSecondary } from 'design/Button';
import { Platform } from 'design/platform';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

export type DownloadLink = { text: string; url: string };

export const DownloadConnect = (props: {
  downloadLinks: Array<DownloadLink>;
}) => {
  if (props.downloadLinks.length === 1) {
    const downloadLink = props.downloadLinks[0];
    return (
      <ButtonSecondary as="a" href={downloadLink.url}>
        Download Teleport Connect
      </ButtonSecondary>
    );
  }

  return (
    <MenuButton buttonText="Download Teleport Connect">
      {props.downloadLinks.map(link => (
        <MenuItem key={link.url} as="a" href={link.url}>
          {link.text}
        </MenuItem>
      ))}
    </MenuButton>
  );
};

export function getConnectDownloadLinks(
  platform: Platform,
  proxyVersion: string
): Array<DownloadLink> {
  switch (platform) {
    case Platform.Windows:
      return [
        {
          text: 'Teleport Connect',
          url: `https://cdn.teleport.dev/Teleport Connect Setup-${proxyVersion}.exe`,
        },
      ];
    case Platform.macOS:
      return [
        {
          text: 'Teleport Connect',
          url: `https://cdn.teleport.dev/Teleport Connect-${proxyVersion}.dmg`,
        },
      ];
    case Platform.Linux:
      return [
        {
          text: 'DEB',
          url: `https://cdn.teleport.dev/teleport-connect_${proxyVersion}_amd64.deb`,
        },
        {
          text: 'RPM',
          url: `https://cdn.teleport.dev/teleport-connect-${proxyVersion}.x86_64.rpm`,
        },

        {
          text: 'tar.gz',
          url: `https://cdn.teleport.dev/teleport-connect-${proxyVersion}-x64.tar.gz`,
        },
      ];
  }
}
