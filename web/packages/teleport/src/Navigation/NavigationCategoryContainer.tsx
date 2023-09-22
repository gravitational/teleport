/*
Copyright 2023 Gravitational, Inc.

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
  top: 25px;
  overflow-y: auto;
`;

const TRANSITION_DELAY_OFFSET = 160;
const TRANSITION_TOTAL_TIME = 140;

export function NavigationCategoryContainer(props: NavigationCategoryProps) {
  const features = useFeatures();

  let items = features
    .filter(feature => feature.category === props.category)
    .filter(feature => !feature.parent);

  if (props.category === NavigationCategory.Assist) {
    // Return an empty div for a portal to be created by the assist route
    // This allows for the conversation history to be put in the sidebar.
    return (
      <Container visible={props.visible}>
        <div id="assist-sidebar"></div>
      </Container>
    );
  }

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
