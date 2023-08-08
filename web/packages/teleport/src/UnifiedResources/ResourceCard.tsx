/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonBorder,
  ButtonLink,
  ButtonText,
  Flex,
  Label,
  Text,
} from 'design';

import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';
import {
  ApplicationsIcon,
  DatabasesIcon,
  DesktopsIcon,
  KubernetesIcon,
  ServersIcon,
} from 'design/SVGIcon';

import {
  AgentLabel,
  UnifiedResource,
  UnifiedResourceKind,
} from 'teleport/services/agents';

const labelRowHeight = 26; // px
const labelVerticalMargin = 1; // px
const labelHeight = labelRowHeight - 2 * labelVerticalMargin;

const SingleLineBox = styled(Box)`
  overflow: hidden;
  white-space: nowrap;
`;

const ResourceLabel = styled(Label)`
  height: ${labelHeight}px;
  margin: 1px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
`;

/**
 * This box serves twofold purpose: first, it prevents the underlying icon from
 * being squeezed if the parent flexbox starts shrinking items. Second, it
 * prevents the icon from magically occupying too much space, since the SVG
 * element somehow forces the parent to occupy at least full line height.
 */
const ResTypeIconBox = styled(Box)`
  line-height: 0;
`;

type Props = {
  resource: UnifiedResource;
  onLabelClick: (label: AgentLabel) => void;
};

export const ResourceCard = ({ resource, onLabelClick }: Props) => {
  const name = resourceName(resource);
  const resIcon = resourceIconName(resource);
  const ResTypeIcon = resourceTypeIcon(resource.kind);
  const description = resourceDescription(resource);
  const labelsInnerContainer = React.useRef(null);
  const [showMoreLabelsButton, setShowMoreLabelsButton] = React.useState(false);
  const [showAllLabels, setShowAllLabels] = React.useState(false);
  const [numMoreLabels, setNumMoreLabels] = React.useState(0);

  React.useEffect(() => {
    if (!labelsInnerContainer.current) return;

    const observer = new ResizeObserver(entries => {
      const container = entries[0];

      // We're taking labelRowHeight * 1.5 just in case some glitch adds or
      // removes a pixel here and there.
      const moreThanOneRow =
        container.contentBoxSize[0].blockSize > labelRowHeight * 1.5;
      setShowMoreLabelsButton(moreThanOneRow);

      // Count number of labels in the first row.
      const labelElements = [
        ...entries[0].target.querySelectorAll('[data-is-label]'),
      ];
      const firstLabelPos = labelElements[0]?.getBoundingClientRect().top;
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
  });

  const onMoreLabelsClick = () => {
    setShowAllLabels(true);
  };

  return (
    <CardContainer>
      <CardInnerContainer
        p={3}
        alignItems="start"
        showAllLabels={showAllLabels}
        onMouseLeave={() => setShowAllLabels(false)}
        // Class name needed to properly propagate hover state to child
        // components. If we don't do it, there's no way to sync the background
        // of the "more" button, since it's opaque and the background is
        // animated.
        className="grv-unified-resource-card"
      >
        <ResourceIcon
          name={resIcon}
          width="45px"
          height="45px"
          ml={2}
          // We would love to just vertical-center-align this one, but then it
          // would move down along with expanding the labels. So we apply a
          // carefully measured top margin instead.
          mt="16px"
        />
        {/* MinWidth is important to prevent descriptions from overflowing. */}
        <Flex flexDirection="column" flex="1" minWidth="0" ml={3} gap={1}>
          <Flex flexDirection="row" alignItems="start">
            <SingleLineBox flex="1" title={name}>
              <Text typography="h5">{name}</Text>
            </SingleLineBox>
            <ButtonBorder size="small">Connect</ButtonBorder>
          </Flex>
          <Flex flexDirection="row" alignItems="center">
            <ResTypeIconBox>
              <ResTypeIcon size={18} />
            </ResTypeIconBox>
            {description.primary && (
              <SingleLineBox ml={1} title={description.primary}>
                <Text typography="body2" color="text.slightlyMuted">
                  {description.primary}
                </Text>
              </SingleLineBox>
            )}
            {description.secondary && (
              <SingleLineBox ml={2} title={description.secondary}>
                <Text typography="body2" color="text.muted">
                  {description.secondary}
                </Text>
              </SingleLineBox>
            )}
          </Flex>
          <LabelsContainer showAll={showAllLabels}>
            <LabelsInnerContainer ref={labelsInnerContainer}>
              <MoreLabelsButton
                style={{
                  visibility:
                    showMoreLabelsButton && !showAllLabels
                      ? 'visible'
                      : 'hidden',
                }}
                onClick={onMoreLabelsClick}
              >
                + {numMoreLabels} more
              </MoreLabelsButton>
              {resource.labels.map(label => {
                const { name, value } = label;
                const labelText = `${name}: ${value}`;
                return (
                  <ResourceLabel
                    key={labelText}
                    title={labelText}
                    onClick={() => onLabelClick?.(label)}
                    kind="secondary"
                    data-is-label=""
                  >
                    {labelText}
                  </ResourceLabel>
                );
              })}
            </LabelsInnerContainer>
          </LabelsContainer>
        </Flex>
      </CardInnerContainer>
    </CardContainer>
  );
};

function resourceName(resource: UnifiedResource) {
  return resource.kind === 'node' ? resource.hostname : resource.name;
}

function resourceDescription(resource: UnifiedResource) {
  switch (resource.kind) {
    case 'app':
      return {
        primary: resource.addrWithProtocol,
        secondary: resource.description,
      };
    case 'db':
      return { primary: resource.type, secondary: resource.description };
    case 'kube_cluster':
      return { primary: 'Kubernetes' };
    case 'node':
      // TODO(bl-nero): Pass the subkind to display as the primary and push addr
      // to secondary.
      return { primary: resource.addr };
    case 'windows_desktop':
      return { primary: 'Windows', secondary: resource.addr };

    default:
      return {};
  }
}

function resourceIconName(resource: UnifiedResource): ResourceIconName {
  switch (resource.kind) {
    case 'app':
      return 'Application';
    case 'db':
      return 'Database';
    case 'kube_cluster':
      return 'Kube';
    case 'node':
      return 'Server';
    case 'windows_desktop':
      return 'Windows';

    default:
      return 'Server';
  }
}

function resourceTypeIcon(kind: UnifiedResourceKind) {
  switch (kind) {
    case 'app':
      return ApplicationsIcon;
    case 'db':
      return DatabasesIcon;
    case 'kube_cluster':
      return KubernetesIcon;
    case 'node':
      return ServersIcon;
    case 'windows_desktop':
      return DesktopsIcon;

    default:
      return ServersIcon;
  }
}

const CardContainer = styled(Box)`
  position: relative;
`;

const CardInnerContainer = styled(Flex)`
  border-top: 2px solid ${props => props.theme.colors.spotBackground[0]};
  background-color: ${props => props.theme.colors.levels.sunken};

  ${props =>
    props.showAllLabels
      ? 'position: absolute; left: 0; right: 0; z-index: 1;'
      : ''}

  transition: all 150ms;

  &:hover {
    background-color: ${props => props.theme.colors.levels.elevated};
    border-color: ${props => props.theme.colors.levels.elevated};
    box-shadow: ${props => props.theme.boxShadow[1]};
  }

  @media (min-width: ${props => props.theme.breakpoints.tablet}px) {
    border: ${props => props.theme.borders[2]}
      ${props => props.theme.colors.spotBackground[0]};
    border-radius: ${props => props.theme.radii[3]}px;
  }
`;

const LabelsContainer = styled(Box)`
  ${props => (props.showAll ? '' : `height: ${labelRowHeight}px;`)}
  overflow: hidden;
`;

const LabelsInnerContainer = styled(Flex)`
  gap: ${props => props.theme.space[1]}px;
  flex-wrap: wrap;
  align-items: start;
  position: relative;
`;

const MoreLabelsButton = styled(ButtonLink)`
  background-color: ${props => props.theme.colors.levels.sunken};
  color: ${props => props.theme.colors.text.slightlyMuted};
  height: ${labelHeight}px;
  margin: ${labelVerticalMargin}px 0;
  min-height: 0;
  font-style: italic;
  border-radius: 0;
  position: absolute;
  right: 0;

  transition: visibility 0s;
  transition: background 150ms;

  .grv-unified-resource-card:hover & {
    background-color: ${props => props.theme.colors.levels.elevated};
  }
`;
