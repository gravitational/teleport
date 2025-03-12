/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { Link as InternalLink } from 'react-router-dom';

import { Mark } from 'design';
import { OutlineInfo } from 'design/Alert/Alert';

import cfg from 'teleport/config';

export const SingleEc2InstanceInstallation = () => (
  <OutlineInfo mt={3} linkColor="buttons.link.default">
    Auto discovery will enroll all EC2 instances found in a region. If you want
    to enroll a <Mark>single</Mark> EC2 instance instead, consider following the{' '}
    <InternalLink
      to={{
        pathname: cfg.routes.discover,
        state: { searchKeywords: 'linux' },
      }}
    >
      Teleport service installation
    </InternalLink>{' '}
    flow.
  </OutlineInfo>
);
