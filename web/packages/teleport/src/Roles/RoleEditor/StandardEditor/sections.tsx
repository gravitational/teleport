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

import { Fragment, PropsWithChildren, useEffect, useId, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import Box, { BoxProps } from 'design/Box';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import { Check, ChevronDown, Trash, WarningCircle } from 'design/Icon';
import Text, { H3 } from 'design/Text';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import { useResizeObserver } from 'design/utils/useResizeObserver';
import { HelperTextLine } from 'shared/components/FieldInput/FieldInput';
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

enum ExpansionState {
  /** The section is fully collapsed. */
  Collapsed,
  /**
   * The section is open, but the expander itself has a zero height. In this
   * state, the content's height can be measured, but the expander still
   * doesn't allow it to be shown. This intermediary step is necessary to allow
   * transition animation to run on Safari.
   */
  Measuring,
  /** The section is fully expanded. */
  Expanded,
  /**
   * The section is still expanded, but it's showing a collapsing animation.
   * In this state, the <details> element is still open to make all its
   * children visible.
   */
  Collapsing,
}

const sectionBoxBorderWidth = 1;

const sectionBoxPadding = 3;

/**
 * A wrapper for editor section. Its responsibility is rendering a header,
 * expanding, collapsing, and removing the section.
 */
export const SectionBox = ({
  titleSegments,
  tooltip,
  children,
  removable,
  isProcessing = false,
  validation,
  onRemove,
  initiallyCollapsed = false,
}: React.PropsWithChildren<{
  titleSegments: string[];
  tooltip?: string;
  removable?: boolean;
  isProcessing?: boolean;
  validation?: ValidationResult;
  onRemove?(): void;
  initiallyCollapsed?: boolean;
}>) => {
  const theme = useTheme();
  const [expansionState, setExpansionState] = useState(
    initiallyCollapsed ? ExpansionState.Collapsed : ExpansionState.Expanded
  );
  const expandTooltip =
    expansionState === ExpansionState.Collapsed ? 'Collapse' : 'Expand';
  const validator = useValidation();
  const [contentHeight, setContentHeight] = useState(0);
  const helperTextId = useId();

  // Points to the content element whose height will be observed for setting
  // target height of the expand animation.
  const contentRef = useResizeObserver(entry => {
    setContentHeight(entry.borderBoxSize[0].blockSize);
  });

  useEffect(() => {
    // After the content is rendered and measured, immediately transition to
    // the Expanded state.
    if (expansionState === ExpansionState.Measuring) {
      setExpansionState(ExpansionState.Expanded);
    }
  }, [expansionState]);

  // Handles expand/collapse clicks.
  const handleExpand = (e: React.MouseEvent) => {
    // Don't let <summary> handle the event, we'll do it ourselves to keep
    // track of the state.
    e.preventDefault();

    setExpansionState(
      expansionState === ExpansionState.Expanded
        ? ExpansionState.Collapsing
        : ExpansionState.Measuring
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
      border={sectionBoxBorderWidth}
      borderColor={
        validator.state.validating && !validation.valid
          ? theme.colors.interactive.solid.danger.default
          : theme.colors.interactive.tonal.neutral[0]
      }
      borderRadius={3}
      role="group"
      aria-describedby={helperTextId}
    >
      <Summary
        height="56px"
        alignItems="center"
        ml={3}
        mr={2}
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
        <Flex flex="1" gap={2} alignItems="center">
          {/* Ensures that the contents are laid out as a single line of text. */}
          <span>
            <InlineH3>
              {expansionState === ExpansionState.Collapsed
                ? titleSegments.map((seg, i) => (
                    <Fragment key={i}>
                      {i > 0 && (
                        <>
                          {' '}
                          <Text
                            as="span"
                            typography="body1"
                            color={theme.colors.text.muted}
                          >
                            /
                          </Text>{' '}
                        </>
                      )}
                      {seg}
                    </Fragment>
                  ))
                : titleSegments[0]}
            </InlineH3>
            {/* Depending on the number of segments, the header will either
                contain an element of `body1` typography or not. It thus may
                have variable height. This is just about one pixel, but it's
                visible enough if this depends on whether the section is
                expanded or closed; the UI jumps up and down in such case. We
                could use center alignment, but it's more correct to use
                baseline alignment in this context. To compensate, we add a
                zero-width space character and style it with `body1`. It forces
                the entire header to always occupy the same amount of space.
                Putting it outside the H3 element makes it less confusing for
                the testing library that won't see this weird thing as a part
                of the element's accessible name.
                */}
            <Text as="span" typography="body1">
              &#8203;
            </Text>
          </span>
          {tooltip && <IconTooltip>{tooltip}</IconTooltip>}
          {validator.state.validating &&
            (validation.valid ? (
              <Check
                size="medium"
                color={theme.colors.interactive.solid.success.default}
              />
            ) : (
              <WarningCircle
                size="medium"
                color={theme.colors.interactive.solid.danger.default}
              />
            ))}
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
      </Summary>
      {/* This element is the one being animated when the section is expanded
          or collapsed. */}
      <ContentExpander
        height={contentExpanderHeight(contentHeight, expansionState)}
        onTransitionEnd={handleContentExpanderTransitionEnd}
      >
        {/* This element is measured, so its size must reflect the size of
            children. */}
        <Box px={sectionBoxPadding} pb={sectionBoxPadding} ref={contentRef}>
          {children}
          <Box
            mt={
              validator.state.validating &&
              !validation.valid &&
              validation.message
                ? 2
                : 0
            }
          >
            <HelperTextLine
              hasError={validator.state.validating && !validation.valid}
              errorMessage={validation.message}
              helperTextId={helperTextId}
            />
          </Box>
        </Box>
      </ContentExpander>
    </Box>
  );
};

function contentExpanderHeight(
  contentHeight: number,
  expansionState: ExpansionState
) {
  // `contentHeight` is 0 when it's not yet known. In this case, don't
  // explicitly set the height; it will only cause a spurious opening animation
  // after the first measurement is made.
  if (contentHeight === 0) {
    return undefined;
  }
  return expansionState === ExpansionState.Expanded ? contentHeight : 0;
}

const Summary = styled(Flex).attrs({ as: 'summary' })`
  cursor: pointer;
  &::-webkit-details-marker {
    display: none;
  }
`;

const ExpandIcon = styled(ChevronDown)<{ expanded: boolean }>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'none' : 'rotate(-90deg)')};
`;

const ContentExpander = styled(Box)`
  transition: all 0.2s ease-in-out;
  overflow: hidden;
`;

const InlineH3 = styled(H3)`
  display: inline;
`;

/**
 * A utility container that applies a horizontal padding that is consistent
 * with the offset of the section content.
 */
export const SectionPadding = (props: PropsWithChildren<BoxProps>) => (
  <Box
    px={sectionBoxPadding}
    borderLeft={sectionBoxBorderWidth}
    borderRight={sectionBoxBorderWidth}
    borderColor="transparent"
    {...props}
  />
);
