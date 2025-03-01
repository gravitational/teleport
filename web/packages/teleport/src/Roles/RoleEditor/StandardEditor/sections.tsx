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

import { useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import { ChevronDown, Trash } from 'design/Icon';
import { H3 } from 'design/Text';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import { useResizeObserver } from 'design/utils/useResizeObserver';
import { useValidation } from 'shared/components/Validation';
import { ValidationResult } from 'shared/components/Validation/rules';

import { StandardModelDispatcher } from './useStandardModel';

/** Properties of a section that uses plain callbacks to change the model. */
export type SectionProps<Model, ValidationResult> = {
  value: Model;
  isProcessing: boolean;
  validation?: ValidationResult;
  onChange(value: Model): void;
};

/** Properties of a section that uses a dispatcher to change the model. */
export type SectionPropsWithDispatch<Model, ValidationResult> = {
  value: Model;
  isProcessing: boolean;
  validation?: ValidationResult;
  dispatch: StandardModelDispatcher;
};

/**
 * A wrapper for editor section. Its responsibility is rendering a header,
 * expanding, collapsing, and removing the section.
 */
export const SectionBox = ({
  title,
  tooltip,
  children,
  removable,
  isProcessing,
  validation,
  onRemove,
}: React.PropsWithChildren<{
  title: string;
  tooltip: string;
  removable?: boolean;
  isProcessing: boolean;
  validation?: ValidationResult;
  onRemove?(): void;
}>) => {
  enum ExpansionState {
    /** The section is fully collapsed. */
    Collapsed,
    /** The section is fully expanded. */
    Expanded,
    /**
     * The section is still expanded, but it's showing a collapsing animation.
     * In this state, the <details> element is still open to make all its
     * children visible.
     */
    Collapsing,
  }

  const theme = useTheme();
  const [expansionState, setExpansionState] = useState(ExpansionState.Expanded);
  const expandTooltip =
    expansionState === ExpansionState.Collapsed ? 'Collapse' : 'Expand';
  const validator = useValidation();
  // Points to the content element whose height will be observed for setting
  // target height of the expand animation.
  const contentRef = useRef();
  const [contentHeight, setContentHeight] = useState(0);

  useResizeObserver(
    contentRef,
    entry => {
      setContentHeight(entry.borderBoxSize[0].blockSize);
    },
    { enabled: true }
  );

  // Handles expand/collapse clicks.
  const handleExpand = (e: React.MouseEvent) => {
    // Don't let <summary> handle the event, we'll do it ourselves to keep
    // track of the state.
    e.preventDefault();

    setExpansionState(
      expansionState === ExpansionState.Expanded
        ? ExpansionState.Collapsing
        : ExpansionState.Expanded
    );
  };

  // Triggered when the collapse animation is finished and we can finally make
  // the <details> element closed.
  const handleContentExpanderTransitionEnd = () => {
    if (expansionState === ExpansionState.Collapsing) {
      setExpansionState(ExpansionState.Collapsed);
    }
  };

  const handleRemove = (e: React.MouseEvent) => {
    // Don't let <summary> handle the event.
    e.stopPropagation();
    onRemove?.();
  };

  return (
    <Box
      as="details"
      open={expansionState !== ExpansionState.Collapsed}
      border={1}
      borderColor={
        validator.state.validating && !validation.valid
          ? theme.colors.interactive.solid.danger.default
          : theme.colors.interactive.tonal.neutral[0]
      }
      borderRadius={3}
      role="group"
    >
      <Flex
        as="summary"
        height="56px"
        alignItems="center"
        ml={3}
        mr={2}
        css={`
          cursor: pointer;
          &::-webkit-details-marker {
            display: none;
          }
        `}
        onClick={handleExpand}
      >
        <HoverTooltip tipContent={expandTooltip}>
          <ExpandIcon
            expanded={expansionState === ExpansionState.Expanded}
            mr={2}
            size="small"
            color={theme.colors.text.muted}
          />
        </HoverTooltip>
        {/* TODO(bl-nero): Show validation result in the summary. */}
        <Flex flex="1" gap={2}>
          <H3>{title}</H3>
          {tooltip && <IconTooltip>{tooltip}</IconTooltip>}
        </Flex>
        {removable && (
          <Box>
            <HoverTooltip tipContent="Remove section">
              <ButtonIcon
                aria-label="Remove section"
                disabled={isProcessing}
                onClick={handleRemove}
              >
                <Trash
                  size="small"
                  color={theme.colors.interactive.solid.danger.default}
                />
              </ButtonIcon>
            </HoverTooltip>
          </Box>
        )}
      </Flex>
      {/* This element is the one being animated when the section is expanded
          or collapsed. */}
      <ContentExpander
        height={expansionState === ExpansionState.Expanded ? contentHeight : 0}
        onTransitionEnd={handleContentExpanderTransitionEnd}
      >
        {/* This element is measured, so its size must reflect the size of
            children. */}
        <Box px={3} pb={3} ref={contentRef}>
          {children}
        </Box>
      </ContentExpander>
    </Box>
  );
};

const ExpandIcon = styled(ChevronDown)<{ expanded: boolean }>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'none' : 'rotate(-90deg)')};
`;

const ContentExpander = styled(Box)`
  transition: all 0.2s ease-in-out;
  overflow: hidden;
`;
