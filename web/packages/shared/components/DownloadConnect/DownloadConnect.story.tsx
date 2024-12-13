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

import { Box, Text } from 'design';
import { Platform } from 'design/platform';

import { DownloadConnect, getConnectDownloadLinks } from './DownloadConnect';

export default {
  title: 'Shared/DownloadConnect',
};

const links = getConnectDownloadLinks(Platform.Windows, '15.1.2');
const macLinks = getConnectDownloadLinks(Platform.macOS, '15.1.2');
const linuxLinks = getConnectDownloadLinks(Platform.Linux, '15.1.2');

export const Story = () => {
  return (
    <Box>
      <Box mb={4}>
        <Text>Single Link (Windows)</Text>
        <DownloadConnect downloadLinks={links} />
      </Box>
      <Box mb={4}>
        <Text>Single Link (Macos)</Text>
        <DownloadConnect downloadLinks={macLinks} />
      </Box>
      <Box mb={4}>
        <Text>Multiple Links (Linux)</Text>
        <DownloadConnect downloadLinks={linuxLinks} />
      </Box>
    </Box>
  );
};
