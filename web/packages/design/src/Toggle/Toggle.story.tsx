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

import { useState } from 'react';

import Flex from '../Flex';
import Text from '../Text';
import { Toggle } from './Toggle';

export default {
  title: 'Design/Toggle',
};

export const Default = () => {
  return (
    <Flex flexDirection="column" gap={4}>
      <ToggleRow initialToggled={false} />
      <ToggleRow initialToggled={true} />
    </Flex>
  );
};

const ToggleRow = (props: { initialToggled: boolean }) => {
  const [toggled, setToggled] = useState(props.initialToggled);

  function toggle(): void {
    setToggled(wasToggled => !wasToggled);
  }

  return (
    <Flex gap={3}>
      <div>
        <Text>Enabled</Text>
        <Toggle onToggle={toggle} isToggled={toggled} />
      </div>
      <div>
        <Text>Disabled</Text>
        <Toggle disabled={true} onToggle={toggle} isToggled={toggled} />
      </div>
      <div>
        <Text>Enabled (large)</Text>
        <Toggle onToggle={toggle} isToggled={toggled} size="large" />
      </div>
      <div>
        <Text>Disabled (large)</Text>
        <Toggle
          disabled={true}
          onToggle={toggle}
          isToggled={toggled}
          size="large"
        />
      </div>
    </Flex>
  );
};
