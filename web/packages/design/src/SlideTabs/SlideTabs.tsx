/*
Copyright 2022 Gravitational, Inc.

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
import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

function SlideTabs({ tabs, name = 'slide-tab', onChange }: props) {
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    onChange(activeIndex);
  }, [activeIndex]);

  return (
    <Wrapper>
      <TabNav>
        {tabs.map((tabName, tabIndex) => (
          <TabLabel
            htmlFor={`${name}-${tabName}`}
            onClick={() => setActiveIndex(tabIndex)}
            itemCount={tabs.length}
            key={`${tabName}-${tabIndex}`}
          >
            {tabName}
            <TabInput type="radio" name={name} id={`${name}-${tabName}`} />
          </TabLabel>
        ))}
      </TabNav>
      <TabSlider itemCount={tabs.length} activeIndex={activeIndex} />
    </Wrapper>
  );
}

type props = {
  // A list of tab names that you'd like displayed in the list of tabs.
  tabs: string[];
  // The name you'd like to use for the form if using multiple tabs on the page.
  // Default: "slide-tab"
  name?: string;
  // To be notified when the selected tab changes supply it with this fn.
  onChange: (selectedTab: number) => void;
};

const Wrapper = styled.div`
  position: relative;
`;

const TabLabel = styled.label`
  cursor: pointer;
  display: flex;
  justify-content: center;
  padding: 10px;
  width: ${props => 100 / props.itemCount}%;
  z-index: 1; /* Ensures that the label is above the background slider. */
`;

const TabInput = styled.input`
  display: none;
`;

const TabSlider = styled.div`
  background-color: rgba(255, 255, 255, 0.1);
  border-radius: 60px;
  box-shadow: 0px 2px 6px rgba(12, 12, 14, 0.1);
  height: 40px;
  left: calc(${props => (100 / props.itemCount) * props.activeIndex}% + 8px);
  margin: 8px 8px 8px 0;
  position: absolute;
  top: 0;
  transition: all 0.3s ease;
  width: calc(${props => 100 / props.itemCount}% - 16px);
`;

const TabNav = styled.nav`
  align-items: center;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 60px;
  display: flex;
  height: 56px;
  justify-content: space-around;
`;

export default SlideTabs;
