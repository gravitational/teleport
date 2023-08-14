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

import application from 'design/assets/resources/appplication.png';
import desktop from 'design/assets/resources/desktop.png';
import database from 'design/assets/resources/database.png';
import stack from 'design/assets/resources/stack.png';
import { Option } from 'shared/components/Select';

import {
  EmployeeOptionsStrings,
  TeamOptionsStrings,
  TitleOptionsStrings,
} from './types';

export enum EmployeeOptions {
  '0-19',
  '20-199',
  '200-499',
  '500-999',
  '1000-4999',
  '5000+',
}

export const EmployeeSelectOptions: Option<
  EmployeeOptionsStrings,
  EmployeeOptionsStrings
>[] = Object.values(EmployeeOptions)
  .filter(v => !isNaN(Number(v)))
  .map(key => ({
    value: EmployeeOptions[key],
    label: EmployeeOptions[key],
  }));

export enum TeamOptions {
  'Software Engineering',
  'DevOps Engineering',
  'IT',
  'Support',
  'Finance',
  'Legal',
  'Other (free-form field)',
}

export const teamSelectOptions: Option<
  TeamOptionsStrings,
  TeamOptionsStrings
>[] = Object.values(TeamOptions)
  .filter(v => !isNaN(Number(v)))
  .map(key => ({
    value: TeamOptions[key],
    label: TeamOptions[key],
  }));

export enum TitleOptions {
  'Individual contributor',
  'Manager',
  'Director',
  'VP',
  'C-Suite/Owner',
}

export const titleSelectOptions: Option<
  TitleOptionsStrings,
  TitleOptionsStrings
>[] = Object.values(TitleOptions)
  .filter(v => !isNaN(Number(v)))
  .map(key => ({
    value: TitleOptions[key],
    label: TitleOptions[key],
  }));

export const supportedResources = [
  { label: 'Web Applications', image: application },
  { label: 'Windows Desktops', image: desktop },
  { label: 'Server/SSH', image: stack },
  { label: 'Databases', image: database },
  { label: 'Kubernetes', image: stack },
];

export const requiredResourceField = (value: string[]) => () => {
  const valid = !!value.length;
  return {
    valid,
    message: 'Resource is required',
  };
};
