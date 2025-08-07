/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
import { useState } from 'react';

import { TabBorder, TabContainer, TabsContainer } from './Tabs';
import { useSlidingBottomBorderTabs } from './useSlidingBottomBorderTabs';

type StoryProps = {
  withBottomBorder: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Design/Tabs',
  component: Controls,
  argTypes: {
    withBottomBorder: {
      control: { type: 'boolean' },
    },
  },
  // default
  args: {
    withBottomBorder: false,
  },
};
export default meta;

enum Tab {
  Example1,
  Example2,
  Example3,
}

export function Controls(props: StoryProps) {
  const [activeTab, setActiveTab] = useState(Tab.Example1);

  const { borderRef, parentRef } = useSlidingBottomBorderTabs({ activeTab });

  return (
    <>
      <TabsContainer
        ref={parentRef}
        mb={3}
        withBottomBorder={props.withBottomBorder}
      >
        <TabContainer
          data-tab-id={Tab.Example1}
          selected={activeTab === Tab.Example1}
          onClick={() => setActiveTab(Tab.Example1)}
        >
          Example Tab 1
        </TabContainer>
        <TabContainer
          data-tab-id={Tab.Example2}
          selected={activeTab === Tab.Example2}
          onClick={() => setActiveTab(Tab.Example2)}
        >
          Example Tab 2
        </TabContainer>
        <TabContainer
          data-tab-id={Tab.Example3}
          selected={activeTab === Tab.Example3}
          onClick={() => setActiveTab(Tab.Example3)}
        >
          Example Tab 3
        </TabContainer>
        <TabBorder ref={borderRef} />
      </TabsContainer>

      {activeTab === Tab.Example1 && <div>Example 1 content</div>}
      {activeTab === Tab.Example2 && <div>Example 2 content</div>}
      {activeTab === Tab.Example3 && <div>Example 3 content</div>}
    </>
  );
}
