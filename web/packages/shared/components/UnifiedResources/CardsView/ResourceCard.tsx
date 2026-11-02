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

import { Ref, useEffect, useLayoutEffect, useRef, useState } from 'react';
import styled, { css } from 'styled-components';

import { Box, Flex } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';

import {
  BackgroundColorProps,
  getBackgroundColor,
  getStatusBackgroundColor,
} from '../shared/getBackgroundColor';
import { PinButton } from '../shared/PinButton';
import { shouldWarnResourceStatus } from '../shared/StatusInfo';
import { ResourceItemProps } from '../types';
import { labelRowHeight, ResourceCardBody } from './ResourceCardBody';
import { ScopedCardLayout } from './ScopedCardLayout';
import { CardLayoutProps } from './types';
import { WarningRightEdgeBadgeSvg } from './WarningRightEdgeBadgeSvg';

export function ResourceCard({
  onLabelClick,
  pinningSupport,
  pinned,
  pinResource,
  selectResource,
  selected,
  onShowStatusInfo,
  showingStatusInfo,
  viewItem,
  visibleInputFields = {
    pin: true,
    checkbox: true,
    copy: true,
    hoverState: true,
  },
  resourceLabelConfig,
  showResourceSelectedIcon,
  showScope,
}: Omit<ResourceItemProps, 'expandAllLabels'>) {
  const { status } = viewItem;
  const [showMoreLabelsButton, setShowMoreLabelsButton] = useState(false);
  const [showAllLabels, setShowAllLabels] = useState(false);
  const [numMoreLabels, setNumMoreLabels] = useState(0);

  const [hovered, setHovered] = useState(false);

  const innerContainer = useRef<HTMLDivElement | null>(null);
  const labelsInnerContainer = useRef<HTMLDivElement>(null);
  const collapseTimeout = useRef<ReturnType<typeof setTimeout>>(null);

  // This effect installs a resize observer whose purpose is to detect the size
  // of the component that contains all the labels. If this component is taller
  // than the height of a single label row, we show a "+x more" button.
  useLayoutEffect(() => {
    if (!labelsInnerContainer.current) return;

    // TODO(ravicious): Use useResizeObserver instead. Ensure that the callback passed to
    // useResizeObserver has a stable identity.
    const observer = new ResizeObserver(entries => {
      const container = entries[0];

      // In Connect, when a tab becomes active, its outermost DOM element switches from `display:
      // none` to `display: flex`. This callback is then fired with the height reported as zero.
      //
      // As such, when checking whether to show the "More labels" button, we should consider only
      // values other than zero.
      if (container.contentRect.height === 0) {
        return;
      }

      // We're taking labelRowHeight * 1.5 just in case some glitch adds or
      // removes a pixel here and there.
      const moreThanOneRow =
        container.contentBoxSize[0].blockSize > labelRowHeight * 1.5;
      setShowMoreLabelsButton(moreThanOneRow);

      // Count number of labels in the first row. This will let us calculate and
      // show the number of labels left out from the view.
      const labelElements = [
        ...entries[0].target.querySelectorAll('[data-is-label]'),
      ];
      const firstLabelPos = labelElements[0]?.getBoundingClientRect().top;
      // Find the first one below.
      const firstLabelInSecondRow = labelElements.findIndex(
        e => e.getBoundingClientRect().top > firstLabelPos
      );

      setNumMoreLabels(
        firstLabelInSecondRow > 0
          ? labelElements.length - firstLabelInSecondRow
          : 0
      );
    });

    observer.observe(labelsInnerContainer.current);
    return () => {
      observer.disconnect();
    };
  }, []);

  // Clear the timeout on unmount to prevent changing a state of an unmounted
  // component.
  useEffect(() => () => clearTimeout(collapseTimeout.current), []);

  const onMoreLabelsClick = () => {
    setShowAllLabels(true);
  };

  const onMouseLeave = () => {
    // If the user expanded the labels and then scrolled down enough to hide the
    // top of the card, we scroll back up and collapse the labels with a small
    // delay to keep the user from losing focus on the card that they were
    // looking at. The delay is picked by hand, since there's no (easy) way to
    // know when the animation ends.
    if (
      showAllLabels &&
      (innerContainer.current?.getBoundingClientRect().top ?? 0) < 0
    ) {
      innerContainer.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      });
      clearTimeout(collapseTimeout.current);
      collapseTimeout.current = setTimeout(() => setShowAllLabels(false), 700);
    } else {
      // Otherwise, we just collapse the labels immediately.
      setShowAllLabels(false);
    }
  };

  const shouldDisplayStatusWarning = shouldWarnResourceStatus(status);

  const layoutProps: CardLayoutProps = {
    viewItem,
    setHovered,
    showingStatusInfo,
    visibleInputFields,
    showAllLabels,
    shouldDisplayStatusWarning,
    pinned,
    selected,
    innerContainer,
    onMouseLeave,
    selectResource,
    pinResource,
    pinningSupport,
    hovered,
    showResourceSelectedIcon,
    labelsInnerContainer,
    showMoreLabelsButton,
    onMoreLabelsClick,
    numMoreLabels,
    resourceLabelConfig,
    onLabelClick,
    onShowStatusInfo,
  };

  return showScope ? (
    <ScopedCardLayout {...layoutProps} />
  ) : (
    <CardLayout {...layoutProps} />
  );
}

function CardLayout({
  viewItem,
  setHovered,
  showingStatusInfo,
  visibleInputFields,
  showAllLabels,
  shouldDisplayStatusWarning,
  pinned,
  selected,
  innerContainer,
  onMouseLeave,
  selectResource,
  pinResource,
  pinningSupport,
  hovered,
  showResourceSelectedIcon,
  labelsInnerContainer,
  showMoreLabelsButton,
  onMoreLabelsClick,
  numMoreLabels,
  resourceLabelConfig,
  onLabelClick,
  onShowStatusInfo,
}: CardLayoutProps) {
  const { primaryIconName, requiresRequest, scope } = viewItem;

  return (
    <CardContainer
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      showingStatusInfo={showingStatusInfo}
      showHoverState={visibleInputFields.hoverState}
    >
      <CardOuterContainer
        showAllLabels={showAllLabels}
        shouldDisplayWarning={shouldDisplayStatusWarning}
        showHoverState={visibleInputFields.hoverState}
      >
        <CardInnerContainer
          showAllLabels={showAllLabels}
          showHoverState={visibleInputFields.hoverState}
          ref={innerContainer}
          p={3}
          // we set padding left a bit larger so we can have space to absolutely
          // position the pin/checkbox buttons
          pl={visibleInputFields.pin || visibleInputFields.checkbox ? 6 : 2}
          alignItems="start"
          onMouseLeave={onMouseLeave}
          pinned={pinned}
          requiresRequest={requiresRequest}
          selected={selected}
          showingStatusInfo={showingStatusInfo}
          shouldDisplayWarning={shouldDisplayStatusWarning}
          // we set extra padding to push contents to the right to make
          // space for the WarningRightEdgeBadgeIcon.
          {...(shouldDisplayStatusWarning && !showAllLabels && { pr: '35px' })}
          {...(shouldDisplayStatusWarning && showAllLabels && { pr: '7px' })}
        >
          {visibleInputFields.checkbox && (
            <CheckboxInput
              checked={selected}
              onChange={selectResource}
              style={{ position: 'absolute', top: '16px', left: '16px' }}
            />
          )}
          {visibleInputFields.pin && (
            <Box
              css={`
                position: absolute;
                // we position far from the top so the layout of the pin doesn't change if we expand the card
                top: ${props => props.theme.space[9]}px;
                transition: none;
                left: 16px;
              `}
            >
              <PinButton
                setPinned={pinResource}
                pinned={pinned}
                pinningSupport={pinningSupport}
                hovered={hovered}
                scoped={!!scope}
              />
            </Box>
          )}
          <ResourceIcon
            name={primaryIconName}
            width="45px"
            height="45px"
            ml={2}
            css={`
              opacity: ${requiresRequest ? '0.5' : '1'};
            `}
          />
          <ResourceCardBody
            viewItem={viewItem}
            visibleInputFields={visibleInputFields}
            hovered={hovered}
            showResourceSelectedIcon={showResourceSelectedIcon}
            showAllLabels={showAllLabels}
            labelsInnerContainer={labelsInnerContainer}
            shouldDisplayStatusWarning={shouldDisplayStatusWarning}
            showMoreLabelsButton={showMoreLabelsButton}
            onMoreLabelsClick={onMoreLabelsClick}
            numMoreLabels={numMoreLabels}
            resourceLabelConfig={resourceLabelConfig}
            onLabelClick={onLabelClick}
          />
          {shouldDisplayStatusWarning && !showAllLabels && (
            <HoverTooltip tipContent={'Show Connection Issue'} placement="left">
              <WarningRightEdgeBadgeIcon onClick={onShowStatusInfo} />
            </HoverTooltip>
          )}
        </CardInnerContainer>
      </CardOuterContainer>
      {/* This is to let the WarningRightEdgeBadgeIcon stay in place while the
        InnerContainer pops out and expands vertically from rendering all
        labels. */}
      {shouldDisplayStatusWarning && showAllLabels && (
        <WarningRightEdgeBadgeIcon />
      )}
    </CardContainer>
  );
}

const WarningRightEdgeBadgeIcon = ({
  onClick,
  ref,
}: {
  onClick?(): void;
  ref?: Ref<HTMLDivElement>;
}) => {
  return (
    <Box
      onClick={onClick}
      ref={ref}
      css={`
        position: absolute;
        top: 0;
        right: 0;
        cursor: pointer;
        height: 100%;
      `}
    >
      <WarningRightEdgeBadgeSvg />
    </Box>
  );
};

const CardContainer = styled.li<{
  showingStatusInfo: boolean;
  showHoverState: boolean;
}>`
  height: 110px;
  list-style: none;

  position: relative;
  .resource-health-status-svg {
    width: 100%;
    height: 100%;

    fill: ${p =>
      p.showingStatusInfo
        ? p.theme.colors.interactive.solid.alert.active
        : p.theme.colors.interactive.solid.alert.default};
  }

  ${p =>
    p.showHoverState &&
    css`
      &:hover {
        .resource-health-status-svg {
          fill: ${p => p.theme.colors.interactive.solid.alert.hover};
        }
      }
    `}
`;

/**
 * The outer container's purpose is to reserve horizontal space on the resource
 * grid. It holds the inner container that normally holds a regular layout of
 * the card, and is fully contained inside the outer container.  Once the user
 * clicks the "more" button, the inner container "pops out" by changing its
 * position to absolute.
 *
 * The card height is fixed to allow the WarningRightEdgeBadgeIcon to stay in
 * place when user clicks on "showAllLabels". Without the fixed height, the
 * container's height shrinks when the inner container pops out, resulting in
 * the svg to jump around (from size difference) and or disappearing.
 */
const CardOuterContainer = styled(Box)<{
  showAllLabels?: boolean;
  shouldDisplayWarning: boolean;
  showHoverState: boolean;
}>`
  border-radius: ${({ theme }) => theme.radii[3]}px;
  transition: all 150ms;

  ${({ showAllLabels, shouldDisplayWarning }) =>
    showAllLabels &&
    css`
      position: absolute;
      left: 0;
      // The padding is required to show the WarningRightEdgeBadgeIcon
      right: ${shouldDisplayWarning ? '28px' : 0};
      z-index: 1;
    `}

  ${({ showHoverState, showAllLabels, theme }) =>
    (showHoverState || showAllLabels) &&
    css`
      ${CardContainer}:hover & {
        background-color: ${theme.colors.levels.surface};
        box-shadow: ${theme.boxShadow[3]};
      }
    `}
`;

/**
 * The inner container that normally holds a regular layout of the card, and is
 * fully contained inside the outer container.  Once the user clicks the "more"
 * button, the inner container "pops out" by changing its position to absolute.
 *
 * TODO(bl-nero): Known issue: this doesn't really work well with one-column
 * layout; we may need to globally set the card height to fixed size on the
 * outer container.
 */
const CardInnerContainer = styled(Flex)<
  BackgroundColorProps & { showHoverState: boolean; showAllLabels?: boolean }
>`
  border: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.spotBackground[0]};
  border-radius: ${props => props.theme.radii[3]}px;
  background-color: ${props => getBackgroundColor(props)};

  ${p =>
    p.shouldDisplayWarning &&
    css`
      border: 2px solid ${p.theme.colors.interactive.solid.alert.default};
      background-color: ${getStatusBackgroundColor({
        showingStatusInfo: p.showingStatusInfo,
        theme: p.theme,
        action: '',
        viewType: 'card',
      })};
    `}

  ${p =>
    p.showingStatusInfo &&
    css`
      border: 2px solid ${p.theme.colors.interactive.solid.alert.active};
    `}

  &:hover {
    // Make the border invisible instead of removing it, this is to prevent things from shifting due to the size change.
    ${p =>
      (p.showHoverState || p.showAllLabels) &&
      css`
        border: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);
      `}

    ${p =>
      p.shouldDisplayWarning &&
      p.showHoverState &&
      css`
        border-color: ${p.theme.colors.interactive.solid.alert.hover};
        background-color: ${getStatusBackgroundColor({
          showingStatusInfo: p.showingStatusInfo,
          theme: p.theme,
          action: 'hover',
          viewType: 'card',
        })};
      `}
  }
`;
