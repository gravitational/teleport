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

import {
  MANAGEMENT_NAVIGATION_SECTIONS,
  NavigationCategory,
} from 'teleport/Navigation/categories';
import { useFeatures } from 'teleport/FeaturesContext';
import { NavigationItem } from 'teleport/Navigation/NavigationItem';
import { NavigationSection } from 'teleport/Navigation/NavigationSection';
import { NavigationItemSize } from 'teleport/Navigation/common';

interface NavigationCategoryProps {
  category: NavigationCategory;
  visible: boolean;
}

interface SelectedProps {
  visible: boolean;
}

const Container = styled.div<SelectedProps>`
  position: absolute;
  width: inherit;
  pointer-events: ${p => (p.visible ? 'auto' : 'none')};
  opacity: ${p => (p.visible ? 1 : 0)};
  transition: opacity 0.15s linear;
  bottom: 0;
  top: 0;
  overflow-y: auto;
`;

const TRANSITION_DELAY_OFFSET = 160;
const TRANSITION_TOTAL_TIME = 140;

export function NavigationCategoryContainer(props: NavigationCategoryProps) {
  const features = useFeatures();

  let items = features
    .filter(feature => feature.category === props.category)
    .filter(feature => !feature.parent);

  if (props.category === NavigationCategory.Resources) {
    const transitionDelayPerItem = TRANSITION_TOTAL_TIME / items.length;

    const children = [];

    let transitionDelay = TRANSITION_DELAY_OFFSET;

    for (const [index, item] of items.entries()) {
      children.push(
        <NavigationItem
          visible={props.visible}
          transitionDelay={transitionDelay}
          feature={item}
          key={index}
          size={NavigationItemSize.Large}
        />
      );

      transitionDelay += transitionDelayPerItem;
    }

    return <Container visible={props.visible}>{children}</Container>;
  }

  const managementSectionsWithItems = MANAGEMENT_NAVIGATION_SECTIONS.filter(
    section => items.filter(feature => feature.section === section).length
  );

  const transitionDelayPerItem =
    TRANSITION_TOTAL_TIME / (items.length + managementSectionsWithItems.length);

  const sections = [];

  let transitionDelay = TRANSITION_DELAY_OFFSET;
  for (const [index, section] of managementSectionsWithItems.entries()) {
    const sectionItems = items.filter(feature => feature.section === section);

    if (!sectionItems.length) {
      continue;
    }

    sections.push(
      <NavigationSection
        visible={props.visible}
        key={index}
        title={section}
        items={sectionItems}
        transitionDelayPerItem={transitionDelayPerItem}
        transitionDelay={transitionDelay}
      />
    );

    transitionDelay += (sectionItems.length + 1) * transitionDelayPerItem;
  }

  return <Container visible={props.visible}>{sections}</Container>;
}
