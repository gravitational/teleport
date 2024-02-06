/**
 Copyright 2023 Gravitational, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

import React, { useState } from 'react';

import Text from '../Text';
import Flex from '../Flex';

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
