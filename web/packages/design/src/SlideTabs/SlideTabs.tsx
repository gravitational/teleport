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

import React from 'react';
import styled from 'styled-components';

import { Indicator, Text } from '..';

export function SlideTabs({
  appearance = 'square',
  activeIndex = 0,
  name = 'slide-tab',
  onChange,
  size = 'xlarge',
  tabs,
  isProcessing = false,
}: props) {
  return (
    <Wrapper>
      <TabNav role="tablist" appearance={appearance} size={size}>
        {tabs.map((tabName, tabIndex) => {
          const selected = tabIndex === activeIndex;
          return (
            <TabLabel
              role="tab"
              htmlFor={`${name}-${tabName}`}
              onClick={e => {
                e.preventDefault();
                onChange(tabIndex);
              }}
              itemCount={tabs.length}
              key={`${tabName}-${tabIndex}`}
              className={tabIndex === activeIndex && 'selected'}
              processing={isProcessing}
            >
              <Box>
                {selected && isProcessing && (
                  <Spinner delay="none" size="25px" />
                )}
                <Text ml={2}>{tabName}</Text>
              </Box>
              <TabInput type="radio" name={name} id={`${name}-${tabName}`} />
            </TabLabel>
          );
        })}
      </TabNav>
      <TabSlider
        itemCount={tabs.length}
        activeIndex={activeIndex}
        appearance={appearance}
        size={size}
      />
    </Wrapper>
  );
}

type props = {
  /**
   * The style to render the selector in.
   */
  appearance?: 'square' | 'round';
  /**
   * The index that you'd like to select on the initial render.
   */
  activeIndex: number;
  /**
   * The name you'd like to use for the form if using multiple tabs on the page.
   * Default: "slide-tab"
   */
  name?: string;
  /**
   * To be notified when the selected tab changes supply it with this fn.
   */
  onChange: (selectedTab: number) => void;
  /**
   * The size to render the selector in.
   */
  size?: 'xlarge' | 'medium';
  /**
   * A list of tab names that you'd like displayed in the list of tabs.
   */
  tabs: string[];
  /**
   * If true, renders a spinner and disables clicking on the tabs.
   *
   * Currently, a spinner is used in absolute positioning which can render
   * outside of the given tab when browser width is narrow enough.
   * Look into horizontal progress bar (connect has one in LinearProgress.tsx)
   */
  isProcessing?: boolean;
};

export type TabComponent = {
  name: string;
  component: React.ReactNode;
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
  opacity: ${p => (p.processing ? 0.5 : 1)};
  pointer-events: ${p => (p.processing ? 'none' : 'auto')};
`;

const TabInput = styled.input`
  display: none;
`;

const TabSlider = styled.div`
  background-color: ${({ theme }) => theme.colors.brand};
  border-radius: ${props => (props.appearance === 'square' ? '8px' : '60px')};
  box-shadow: 0px 2px 6px rgba(12, 12, 14, 0.1);
  height: ${props => (props.size === 'xlarge' ? '56px' : '40px')};
  left: calc(${props => (100 / props.itemCount) * props.activeIndex}% + 8px);
  margin: ${props =>
    props.size === 'xlarge' ? '12px 12px 12px 0' : '4px 4px 4px 0'};
  position: absolute;
  top: 0;
  transition: all 0.3s ease;
  width: calc(${props => 100 / props.itemCount}% - 16px);
`;

const TabNav = styled.nav`
  align-items: center;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  border-radius: ${props => (props.appearance === 'square' ? '8px' : '60px')};
  display: flex;
  height: ${props => (props.size === 'xlarge' ? '80px' : '47px')};
  justify-content: space-around;
  color: ${props => props.theme.colors.text.main};
  .selected {
    color: ${props => props.theme.colors.text.primaryInverse};
    transition: color 0.2s ease-in 0s;
  }
`;

const Spinner = styled(Indicator)`
  color: ${p => p.theme.colors.levels.deep};
  position: absolute;
  left: -${p => p.theme.space[4]}px;
`;

const Box = styled.div`
  position: relative;
`;
