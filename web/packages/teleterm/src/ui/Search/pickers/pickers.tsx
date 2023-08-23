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

import { ParametrizedAction } from '../actions';

import { ActionPicker } from './ActionPicker';
import { ParameterPicker } from './ParameterPicker';

export const actionPicker: SearchPicker = {
  picker: props => <ActionPicker input={props.input} />,
  placeholder: 'Search for resources by name and labels across clusters',
};
export const getParameterPicker = (
  parametrizedAction: ParametrizedAction
): SearchPicker => {
  return {
    picker: props => (
      <ParameterPicker input={props.input} action={parametrizedAction} />
    ),
    placeholder: parametrizedAction.parameter.placeholder,
  };
};

export interface SearchPicker {
  picker: React.ComponentType<{
    input: React.ReactElement;
  }>;
  placeholder: string;
}
