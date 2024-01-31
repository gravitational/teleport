/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { Prompt } from 'react-router-dom';
import { Box } from 'design';

import { Navigation } from 'teleport/components/Wizard/Navigation';
import { FeatureBox } from 'teleport/components/Layout';
import { SelectResource } from 'teleport/Discover/SelectResource/SelectResource';
import cfg from 'teleport/config';
import { findViewAtIndex } from 'teleport/components/Wizard/flow';

import { EViewConfigs } from './types';

import { DiscoverProvider, useDiscover } from './useDiscover';
import { DiscoverIcon } from './SelectResource/icons';

function DiscoverContent() {
  const {
    currentStep,
    viewConfig,
    onSelectResource,
    indexedViews,
    ...agentProps
  } = useDiscover();

  let content;
  const hasSelectedResource = Boolean(viewConfig);
  if (hasSelectedResource) {
    const view = findViewAtIndex(indexedViews, currentStep);

    const Component = view.component;

    content = <Component {...agentProps} />;

    if (viewConfig.wrapper) {
      content = viewConfig.wrapper(content);
    }
  } else {
    content = (
      <SelectResource onSelect={resource => onSelectResource(resource)} />
    );
  }

  return (
    <>
      <FeatureBox>
        {hasSelectedResource && (
          <Box mt={2} mb={7}>
            <Navigation
              currentStep={currentStep}
              views={indexedViews}
              startWithIcon={{
                title: agentProps.resourceSpec.name,
                component: <DiscoverIcon name={agentProps.resourceSpec.icon} />,
              }}
            />
          </Box>
        )}
        <Box>{content}</Box>
      </FeatureBox>

      {hasSelectedResource && (
        <Prompt
          message={nextLocation => {
            if (nextLocation.pathname === cfg.routes.discover) return true;
            return 'Are you sure you want to exit the "Enroll New Resource” workflow? You’ll have to start from the beginning next time.';
          }}
          when={
            viewConfig.shouldPrompt
              ? viewConfig.shouldPrompt(currentStep, agentProps.resourceSpec)
              : true
          }
        />
      )}
    </>
  );
}

export function DiscoverComponent({ eViewConfigs = [] }: Props) {
  return (
    <DiscoverProvider eViewConfigs={eViewConfigs}>
      <DiscoverContent />
    </DiscoverProvider>
  );
}

export function Discover() {
  return <DiscoverComponent />;
}

type Props = {
  eViewConfigs?: EViewConfigs;
};
