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

import Flex from 'design/Flex';
import Label from 'design/Label/Label';

import { Panel } from './Panel';

export function Roles(props: { roles: string[] }) {
  const { roles } = props;

  return (
    <Panel title="Roles" isSubPanel testId="roles-panel">
      <Flex>
        {roles.map(r => (
          <Label mr="1" key={r} kind="outline">
            {r}
          </Label>
        ))}
      </Flex>
    </Panel>
  );
}
