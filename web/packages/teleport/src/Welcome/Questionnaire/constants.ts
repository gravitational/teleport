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

import application from 'design/assets/resources/appplication.png';
import desktop from 'design/assets/resources/desktop.png';
import database from 'design/assets/resources/database.png';
import kubernetes from 'design/assets/resources/kubernetes.png';
import stack from 'design/assets/resources/stack.png';
import { Option } from 'shared/components/Select';
import { assertUnreachable } from 'shared/utils/assertUnreachable';

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import {
  EmployeeOption,
  ResourceOption,
  TeamOption,
  TitleOption,
} from './types';

export const EmployeeSelectOptions: Option<string, EmployeeOption>[] =
  Object.keys(EmployeeOption).map(key => ({
    value: EmployeeOption[key],
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

export const GetResourceIcon = (key: ResourceOption) => {
  switch (key) {
    case ResourceOption.RESOURCE_WEB_APPLICATIONS:
      return application;
    case ResourceOption.RESOURCE_WINDOWS_DESKTOPS:
      return desktop;
    case ResourceOption.RESOURCE_SERVER_SSH:
      return stack;
    case ResourceOption.RESOURCE_DATABASES:
      return database;
    case ResourceOption.RESOURCE_KUBERNETES:
      return kubernetes;
    default:
      return assertUnreachable(key);
  }
};

export const requiredResourceField = (value: ResourceOption[]) => () => {
  const valid = !!value.length;
  return {
    valid,
    message: 'Resource is required',
  };
};

export const resourceMapping: { [key in ResourceOption]: Resource } = {
  [ResourceOption.RESOURCE_WINDOWS_DESKTOPS]: Resource.WINDOWS_DESKTOPS,
  [ResourceOption.RESOURCE_SERVER_SSH]: Resource.SERVER_SSH,
  [ResourceOption.RESOURCE_DATABASES]: Resource.DATABASES,
  [ResourceOption.RESOURCE_KUBERNETES]: Resource.KUBERNETES,
  [ResourceOption.RESOURCE_WEB_APPLICATIONS]: Resource.WEB_APPLICATIONS,
};
