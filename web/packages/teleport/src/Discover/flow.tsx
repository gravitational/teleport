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

import { ResourceKind } from 'teleport/Discover/Shared';
import { AgentStepComponent } from 'teleport/Discover/types';
import { DiscoverEvent } from 'teleport/services/userEvent';
import { BaseView } from 'teleport/components/Wizard/flow';

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
   * on exit or changing route.
   */
  shouldPrompt?: (currentStep: number, resourceSpec: ResourceSpec) => boolean;
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
