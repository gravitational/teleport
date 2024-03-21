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
