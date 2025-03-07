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

import {
  PropsWithChildren,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import styled from 'styled-components';

import { Box, Button, Text } from 'design';
import { BoxProps } from 'design/Box';
import { Minus, Plus } from 'design/Icon';

type CollapsibleInfoSectionProps = {
  /* defaultOpen is optional and determines whether the section is open or closed initially */
  defaultOpen?: boolean;
  /* size is an optional prop to set the size of the toggle button */
  size?: 'small' | 'large';
  /* onClick is an optional callback for when the toggle is clicked */
  onClick?: (isOpen: boolean) => void;
  /* openLabel is an optional label for the closed state */
  openLabel?: string;
  /* closeLabel is an optional label for the opened state */
  closeLabel?: string;
  /* disabled is an optional flag to disable the toggle */
  disabled?: boolean;
} & BoxProps;

/**
 * CollapsibleInfoSection is a collapsible section that shows more information when expanded.
 * It is useful for hiding less important – or more detailed – information by default.
 */
export const CollapsibleInfoSection = ({
  defaultOpen = false,
  openLabel = 'More info',
  closeLabel = 'Less info',
  size = 'large',
  onClick,
  disabled = false,
  children,
  ...boxProps
}: PropsWithChildren<CollapsibleInfoSectionProps>) => {
  const contentId = useId();
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const [contentHeight, setContentHeight] = useState(0);
  const [shouldRenderContent, setShouldRenderContent] = useState(defaultOpen);
  const contentRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!contentRef.current || !shouldRenderContent) {
      return;
    }
    const ro = new ResizeObserver(entries => {
      for (const entry of entries) {
        if (entry.target === contentRef.current) {
          setContentHeight(entry.contentRect.height);
        }
      }
    });
    ro.observe(contentRef.current);
    return () => ro.disconnect();
  }, [contentRef, shouldRenderContent]);

  return (
    <Box {...boxProps}>
      <ToggleButton
        size={size}
        onClick={() => {
          !isOpen && setShouldRenderContent(true);
          setIsOpen(!isOpen);
          onClick?.(!isOpen);
        }}
        disabled={disabled}
        aria-expanded={isOpen}
        aria-controls={contentId}
      >
        {isOpen ? <Minus size="small" /> : <Plus size="small" />}
        <Text>{isOpen ? closeLabel : openLabel}</Text>
      </ToggleButton>
      <ContentWrapper
        id={contentId}
        role="region"
        aria-hidden={!isOpen}
        $isOpen={isOpen}
        $contentHeight={contentHeight}
        onTransitionEnd={() => !isOpen && setShouldRenderContent(false)}
      >
        {shouldRenderContent && (
          <Box
            ml={size === 'small' ? 3 : 4}
            style={{ position: 'relative' }}
            ref={contentRef}
          >
            <Bar />
            <Box
              ml={3}
              pt={size === 'small' ? 2 : 3}
              pb={size === 'small' ? 1 : 2}
            >
              {children}
            </Box>
          </Box>
        )}
      </ContentWrapper>
    </Box>
  );
};

const ToggleButton = styled(Button).attrs({ intent: 'neutral' })<{
  size: 'small' | 'large';
}>`
  display: flex;
  flex-direction: row;
  align-items: center;
  ${({ size, theme }) =>
    size === 'small'
      ? `padding: ${theme.space[1]}px ${theme.space[2]}px;`
      : `padding: ${theme.space[2]}px ${theme.space[3]}px;`}
  gap: ${({ theme }) => theme.space[2]}px;
`;

const Bar = styled.div`
  position: absolute;
  top: ${({ theme }) => theme.space[1]}px;
  bottom: 0;
  left: 0;
  width: 2px;
  background: ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
`;

const ContentWrapper = styled.div<{ $isOpen: boolean; $contentHeight: number }>`
  overflow: hidden;
  height: ${props => (props.$isOpen ? `${props.$contentHeight}px` : '0')};
  will-change: height;
  transition: height 200ms ease;
  transform-origin: top;
`;
