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

import { BaseView } from 'teleport/components/Wizard/flow';
import { ResourceKind } from 'teleport/Discover/Shared';
import { AgentStepComponent } from 'teleport/Discover/types';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { ResourceSpec } from './SelectResource';

type ViewFunction<T> = (t: T) => View[];

export interface ResourceViewConfig<T = any> {
  kind: ResourceKind;
  /**
   * views contain all the possible views for a resource kind.
   * Resources with no sub types will have views defined
   * in a simple View list (eg. kubernetes and servers).
   * ViewFunction is defined instead if a resource can have
   * varying views depending on the resource "sub-type". For
   * example, a database resource can have many sub-types.
   * A aws postgres will contain different views versus a
   * self-hosted postgres.
   */
  views: View[] | ViewFunction<T>;
  wrapper?: (component: React.ReactNode) => React.ReactNode;
  /**
   * shouldPrompt is an optional function that determines if the
   * react-router-dom's Prompt should be invocated on exit or
   * changing route. We can control when to show the prompt
   * depending on what step in the flow a user is in (indicated
   * by "currentStep" param).
   * Not supplying a function is equivalent to always prompting
   * on exit or changing route when not on a step with the
   * eventName `DiscoverEvent.Completed`.
   */
  shouldPrompt?: (
    currentStep: number,
    currentView: View | undefined,
    resourceSpec: ResourceSpec
  ) => boolean;
}

export type View = BaseView<{
  component?: AgentStepComponent;
  eventName?: DiscoverEvent;
  /**
   * manuallyEmitSuccessEvent is a flag that when true
   * means success events will be sent by the children
   * (current view component) instead of the default
   * which is sent by the parent context.
   */
  manuallyEmitSuccessEvent?: boolean;
}>;
