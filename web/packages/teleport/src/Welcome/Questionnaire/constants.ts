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
  EmployeeOption,
  ProtoResource,
  ResourceOption,
  TeamOption,
  TitleOption,
} from './types';

export const EmployeeSelectOptions: Option<string, EmployeeOption>[] =
  Object.keys(EmployeeOption).map(key => ({
    value: key,
    label: EmployeeOption[key],
  }));

export const teamSelectOptions: Option<string, TeamOption>[] = (
  Object.keys(TeamOption) as Array<keyof typeof TeamOption>
).map(key => ({
  value: key,
  label: TeamOption[key],
}));

export const titleSelectOptions: Option<string, TitleOption>[] = (
  Object.keys(TitleOption) as Array<keyof typeof TitleOption>
).map(key => ({
  value: key,
  label: TitleOption[key],
}));

export const ResourceOptions: Option<string, ResourceOption>[] = Object.keys(
  ResourceOption
).map(key => ({
  value: key,
  label: ResourceOption[key],
}));

export const GetResourceIcon = key => {
  switch (ResourceOption[key]) {
    case ResourceOption.RESOURCE_WEB_APPLICATIONS:
      return application;
    case ResourceOption.RESOURCE_WINDOWS_DESKTOPS:
      return desktop;
    case ResourceOption.RESOURCE_SERVER_SSH:
      return stack;
    case ResourceOption.RESOURCE_DATABASES:
      return database;
    case ResourceOption.RESOURCE_KUBERNETES:
      return stack;
    default:
      return stack;
  }
};

export const requiredResourceField = (value: ResourceOption[]) => () => {
  const valid = !!value.length;
  return {
    valid,
    message: 'Resource is required',
  };
};

export const resourceMapping: { [key in ResourceOption]: ProtoResource } = {
  [ResourceOption.RESOURCE_WINDOWS_DESKTOPS]:
    ProtoResource.RESOURCE_WINDOWS_DESKTOPS,
  [ResourceOption.RESOURCE_SERVER_SSH]: ProtoResource.RESOURCE_SERVER_SSH,
  [ResourceOption.RESOURCE_DATABASES]: ProtoResource.RESOURCE_DATABASES,
  [ResourceOption.RESOURCE_KUBERNETES]: ProtoResource.RESOURCE_KUBERNETES,
  [ResourceOption.RESOURCE_WEB_APPLICATIONS]:
    ProtoResource.RESOURCE_WEB_APPLICATIONS,
};
