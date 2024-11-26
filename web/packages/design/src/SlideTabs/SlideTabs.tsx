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

import React, { useEffect, useRef } from 'react';
import styled from 'styled-components';

import { Flex, Indicator } from 'design';
import { IconProps } from 'design/Icon/Icon';

export function SlideTabs({
  appearance = 'square',
  activeIndex = 0,
  onChange,
  size = 'large',
  tabs,
  isProcessing = false,
  disabled = false,
  fitContent = false,
}: SlideTabsProps) {
  const activeTab = useRef<HTMLButtonElement>(null);
  const tabContainer = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // Note: this is important for accessibility; the screen reader may ignore
    // tab changing if we focus the tab list, and not the tab.
    if (tabContainer.current?.contains?.(document.activeElement)) {
      activeTab.current?.focus();
    }
  }, [activeIndex]);

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'ArrowRight' && activeIndex < tabs.length - 1) {
      onChange(activeIndex + 1);
    }
    if (e.key === 'ArrowLeft' && activeIndex > 0) {
      onChange(activeIndex - 1);
    }
  }

  // The component structure was designed according to
  // https://www.w3.org/WAI/ARIA/apg/patterns/tabs/examples/tabs-automatic/.
  return (
    // A container that displays background and sets up padding for the slider
    // area. It's separate from the tab list itself, since we need to
    // absolutely position the slider relative to this container's content box,
    // and not its padding box. So we set up padding if needed on this one, and
    // then position the slider against the tab list.
    <Wrapper fitContent={fitContent} size={size} appearance={appearance}>
      <TabList
        ref={tabContainer}
        role="tablist"
        itemCount={tabs.length}
        onKeyDown={handleKeyDown}
      >
        {tabs.map((tabSpec, tabIndex) => {
          const selected = tabIndex === activeIndex;
          const {
            key,
            title,
            icon: Icon,
            ariaLabel,
            controls,
          } = toFullTabSpec(tabSpec, tabIndex);

          let onClick = undefined;
          if (!disabled && !isProcessing) {
            onClick = (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => {
              e.preventDefault();
              onChange(tabIndex);
            };
          }

          return (
            <TabButton
              key={key}
              ref={selected ? activeTab : undefined}
              role="tab"
              onClick={onClick}
              selected={selected}
              className={selected ? 'selected' : undefined}
              aria-controls={controls}
              tabIndex={!disabled && selected ? 0 : -1}
              processing={isProcessing}
              disabled={disabled}
              aria-selected={selected}
              size={size}
              aria-label={ariaLabel}
            >
              {/* We need a separate tab content component, since the spinner,
                  when displayed, shouldn't take up space to prevent layout
                  jumping. TabContent serves as a positioning anchor whose left
                  edge is the left edge of the content (not the tab button,
                  which can be much wider). */}
              <TabContent gap={1}>
                {selected && isProcessing && <Spinner delay="none" size={25} />}
                {Icon && <Icon size={size} role="graphics-symbol" />}
                {title}
              </TabContent>
            </TabButton>
          );
        })}
        {/* The tab slider is positioned absolutely and appears below the
            actual tab button. The outer component is responsible for
            establishing the part of parent's width where the slider appears,
            and the internal slider may (or may not, depending on the control
            size) include additional padding that separates tabs. */}
        <TabSlider
          itemCount={tabs.length}
          activeIndex={activeIndex}
          size={size}
        >
          <TabSliderInner appearance={appearance} />
        </TabSlider>
      </TabList>
    </Wrapper>
  );
}

export type SlideTabsProps = {
  /**
   * The style to render the selector in.
   */
  appearance?: 'square' | 'round';
  /**
   * The index that you'd like to select on the initial render.
   */
  activeIndex: number;
  /**
   * To be notified when the selected tab changes supply it with this fn.
   */
  onChange: (selectedTab: number) => void;
  /**
   * The size to render the selector in.
   */
  size?: Size;
  /**
   * A list of tab specs that you'd like displayed in the list of tabs.
   */
  tabs: TabSpec[];
  /**
   * If true, renders a spinner and disables clicking on the tabs.
   *
   * Currently, a spinner is used in absolute positioning which can render
   * outside of the given tab when browser width is narrow enough.
   * Look into horizontal progress bar (connect has one in LinearProgress.tsx)
   */
  isProcessing?: boolean;
  /**
   * If true, disables pointer events.
   */
  disabled?: boolean;
  /**
   * If true, the control doesn't take as much horizontal space as possible,
   * but instead wraps its contents.
   */
  fitContent?: boolean;
};

/**
 * Definition of a tab. If it's a string, it denotes a title displayed on the
 * tab. It's recommended to use a full object with tab panel ID for better
 * accessibility.
 *
 * TODO(bl-nero): remove the string option once Enterprise is migrated to
 * simplify it a bit.
 */
type TabSpec = string | FullTabSpec;

type FullTabSpec = TabContentSpec & {
  /** Iteration key for the tab. */
  key: React.Key;
  /**
   * ID of the controlled element for accessibility, perhaps autogenerated
   * with an `useId()` hook. The indicated element should have a `role`
   * attribute set to "tabpanel".
   */
  controls?: string;
};

/**
 * Tab content. Either an icon with a mandatory accessible label, or a
 * decorative icon with accompanying text.
 */
type TabContentSpec =
  | {
      /** Title displayed on the tab. */
      title: string;
      ariaLabel?: never;
      /** Icon displayed on the tab. */
      icon?: React.ComponentType<IconProps>;
    }
  | {
      title?: never;
      /** Accessible label for the tab. */
      ariaLabel: string;
      /** Icon displayed on the tab. */
      icon: React.ComponentType<IconProps>;
    };

function toFullTabSpec(spec: TabSpec, index: number): FullTabSpec {
  if (typeof spec !== 'string') return spec;
  return {
    key: index,
    title: spec,
  };
}

const TabSliderInner = styled.div<{ appearance: Appearance }>`
  height: 100%;
  background-color: ${({ theme }) => theme.colors.brand};
  border-radius: ${props => (props.appearance === 'square' ? '8px' : '60px')};
`;

const Wrapper = styled.div<{
  fitContent: boolean;
  size: Size;
  appearance: Appearance;
}>`
  position: relative;
  ${props => (props.fitContent ? 'width: fit-content;' : '')}
  /*
   * For the small size, we don't use paddings between tab buttons. Therefore,
   * the area of tab list is evenly divided into segments, and we anchor the
   * slider relative to the box with horizontal padding. With larger sizes, we
   * expect to have some distance between the tab buttons. It means that the
   * positions of the slider, expressed as relative to what the padding box
   * would be, are no longer proportional to the tab index (there is distance
   * between the tabs, but no distance on the left of the first tab and on the
   * right of the last tab). Therefore, to calculate the position of slider as
   * a percentage of its container's width, we set the wrapper's horizontal
   * padding to 0, thus giving us a couple of pixels of breathing room; now we
   * can go back to using a linear formula to calculate the slider position.
   * This lack of padding will be then compensated for by adjusting tab button
   * margins appropriately.
   */
  padding: ${props => (props.size === 'small' ? '4px 4px' : '8px 0')};
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${props => (props.appearance === 'square' ? '8px' : '60px')};

  &:has(:focus-visible) ${TabSliderInner} {
    outline: 2px solid ${props => props.theme.colors.brand};
    outline-offset: 1px;
  }
`;

const tabButtonHeight = ({ size }: { size: Size }) => {
  switch (size) {
    case 'large':
      return { height: '40px' };
    case 'medium':
      return { height: '36px' };
    case 'small':
      return { height: '32px' };
  }
};

const TabButton = styled.button<{
  processing?: boolean;
  disabled?: boolean;
  selected?: boolean;
  size: Size;
}>`
  /* Reset the button styles. */
  font-family: inherit;
  text-decoration: inherit;
  outline: none;
  border: none;
  background: transparent;
  padding: ${props => (props.size === 'small' ? '8px 8px' : '8px 16px')};

  ${props => props.theme.typography.body2}

  cursor: ${p => (p.processing || p.disabled ? 'default' : 'pointer')};
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1; /* Ensures that the label is above the background slider. */
  opacity: ${p => (p.processing || p.disabled ? 0.5 : 1)};
  ${tabButtonHeight}
  /*
   * Using similar logic as with wrapper padding, we compensate for the lack of
   * thereof with button margins if needed.
   */
  margin: 0 ${props => (props.size === 'small' ? '0' : '8px')};
  color: ${props =>
    props.selected
      ? props.theme.colors.text.primaryInverse
      : props.theme.colors.text.main};

  transition: color 0.2s ease-in 0s;
`;

type Appearance = 'square' | 'round';
type Size = 'large' | 'medium' | 'small';

const TabSlider = styled.div<{
  itemCount: number;
  size: Size;
  activeIndex: number;
}>`
  box-sizing: border-box;
  position: absolute;
  left: ${props => (100 / props.itemCount) * props.activeIndex}%;
  top: 0;
  bottom: 0;
  width: ${props => 100 / props.itemCount}%;
  padding: 0 ${props => (props.size === 'small' ? '0' : '8px')};
  transition:
    all 0.3s ease,
    outline 0s,
    outline-offset 0s;
`;

const TabList = styled.div<{ itemCount: number }>`
  position: relative;
  align-items: center;
  /* 
   * Grid display allows us to allocate equal amount of space for every tab
   * (which is important for calculating the slider position) and at the same
   * time support the "fit content" mode. (It's impossible to do in the flex
   * layout.)
   */
  display: grid;
  grid-template-columns: repeat(${props => props.itemCount}, 1fr);
  justify-content: space-around;
  color: ${props => props.theme.colors.text.main};
`;

const Spinner = styled(Indicator)`
  color: ${p => p.theme.colors.levels.deep};
  position: absolute;
  left: -${p => p.theme.space[5]}px;
`;

const TabContent = styled(Flex)`
  position: relative;
`;
