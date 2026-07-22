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

import { TextSelectCopyMulti } from '../TextSelectCopy';
import { Tabs } from './Tabs';

export default {
  title: 'Teleport/Tabs',
};

export const Plain = () => {
  return (
    <Tabs
      tabs={[
        {
          title: `One`,
          content: (
            <div>
              Lorem Ipsum is simply dummy text of the printing and typesetting
              industry.
            </div>
          ),
        },
        {
          title: `Two`,
          content: (
            <div>
              Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do
              eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut
              enim ad minim veniam, quis nostrud exercitation ullamco laboris
              nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in
              reprehenderit in voluptate velit esse cillum dolore eu fugiat
              nulla pariatur.
            </div>
          ),
        },
      ]}
    />
  );
};

export const PlainOneTab = () => {
  return (
    <Tabs
      tabs={[
        {
          title: `One`,
          content: (
            <div>
              Lorem Ipsum is simply dummy text of the printing and typesetting
              industry.
            </div>
          ),
        },
      ]}
    />
  );
};

export const TerminalCommands = () => {
  return (
    <Tabs
      tabs={[
        {
          title: `Debian/Ubuntu (DEB)`,
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users add --roles=admin lisa',
                  comment: 'Some kind of comment',
                },
              ]}
            />
          ),
        },
        {
          title: 'Tarball',
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users add --roles=access,admin georege',
                  comment: 'Lorem ipsum dolores',
                },
                {
                  text: 'tsh login --user=georege',
                },
              ]}
            />
          ),
        },
        {
          title: 'Debian/Ubuntu Legacy (DEB)',
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users add --roles=access,admin georege',
                },
                {
                  text: 'tsh login --user=georege',
                },
              ]}
            />
          ),
        },
        {
          title: 'Amazon Linux 2/RHEL Legacy (RPM)',
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users ls',
                },
              ]}
            />
          ),
        },
      ]}
    />
  );
};
