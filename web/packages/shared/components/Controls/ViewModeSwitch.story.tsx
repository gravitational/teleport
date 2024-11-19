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

import React, { useState } from 'react';
import { Flex } from 'design';

import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import { ViewModeSwitch } from './ViewModeSwitch';

import type { Meta, StoryFn, StoryObj } from '@storybook/react';

export default {
  title: 'Shared/Controls/ViewModeSwitch',
  component: ViewModeSwitch,
  argTypes: {
    currentViewMode: {
      control: { type: 'radio', options: [ViewMode.CARD, ViewMode.LIST] },
      description: 'Current view mode',
      table: { defaultValue: { summary: ViewMode.CARD.toString() } },
    },
    setCurrentViewMode: {
      control: false,
      description: 'Callback to set current view mode',
      table: { type: { summary: '(newViewMode: ViewMode) => void' } },
    },
  },
  args: { currentViewMode: ViewMode.CARD },
  parameters: { controls: { expanded: true, exclude: ['userContext'] } },
} satisfies Meta<typeof ViewModeSwitch>;

const Default: StoryObj<typeof ViewModeSwitch> = {
  render: (({ currentViewMode }) => {
    const [viewMode, setViewMode] = useState(currentViewMode);
    return (
      <Flex alignItems="center" minHeight="100px">
        <ViewModeSwitch
          currentViewMode={viewMode}
          setCurrentViewMode={setViewMode}
        />
      </Flex>
    );
  }) satisfies StoryFn<typeof ViewModeSwitch>,
};

export { Default as ViewModeSwitch };
