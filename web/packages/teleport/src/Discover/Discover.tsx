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
import { useLocation } from 'react-router';
import { Prompt } from 'react-router-dom';

import { Box } from 'design';

import { FeatureBox } from 'teleport/components/Layout';
import { findViewAtIndex } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';
import cfg from 'teleport/config';
import type { View } from 'teleport/Discover/flow';
import { SelectResource } from 'teleport/Discover/SelectResource/SelectResource';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { DiscoverIcon } from './SelectResource/icons';
import { EViewConfigs } from './types';
import {
  DiscoverProvider,
  DiscoverUpdateProps,
  useDiscover,
} from './useDiscover';

function DiscoverContent() {
  const {
    currentStep,
    viewConfig,
    onSelectResource,
    indexedViews,
    ...agentProps
  } = useDiscover();

  let currentView: View | undefined;
  let content: React.ReactNode;

  const hasSelectedResource = Boolean(viewConfig);

  if (hasSelectedResource) {
    currentView = findViewAtIndex(indexedViews, currentStep);

    const Component = currentView.component;

    content = <Component {...agentProps} />;

    if (viewConfig.wrapper) {
      content = viewConfig.wrapper(content);
    }
  } else {
    currentView = undefined;

    content = (
      <SelectResource onSelect={resource => onSelectResource(resource)} />
    );
  }

  return (
    <>
      <FeatureBox>
        {hasSelectedResource && (
          <Box mt={2} mb={6}>
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
              ? viewConfig.shouldPrompt(
                  currentStep,
                  currentView,
                  agentProps.resourceSpec
                )
              : currentView?.eventName !== DiscoverEvent.Completed
          }
        />
      )}
    </>
  );
}

export function DiscoverComponent({
  eViewConfigs = [],
  updateFlow,
}: DiscoverComponentProps) {
  const location = useLocation();
  return (
    <DiscoverProvider
      eViewConfigs={eViewConfigs}
      key={location.key}
      updateFlow={updateFlow}
    >
      <DiscoverContent />
    </DiscoverProvider>
  );
}

export function Discover() {
  return <DiscoverComponent />;
}

export type DiscoverComponentProps = {
  eViewConfigs?: EViewConfigs;
  updateFlow?: DiscoverUpdateProps;
};
