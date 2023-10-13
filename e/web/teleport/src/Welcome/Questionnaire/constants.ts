import application from 'design/assets/resources/appplication.png';
import desktop from 'design/assets/resources/desktop.png';
import database from 'design/assets/resources/database.png';
import kubernetes from 'design/assets/resources/kubernetes.png';
import stack from 'design/assets/resources/stack.png';
import { Option } from 'shared/components/Select';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { ClusterResource } from 'teleport/services/userPreferences/types';

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

export const resourceMapping: { [key in ResourceOption]: ClusterResource } = {
  [ResourceOption.RESOURCE_WINDOWS_DESKTOPS]:
    ClusterResource.RESOURCE_WINDOWS_DESKTOPS,
  [ResourceOption.RESOURCE_SERVER_SSH]: ClusterResource.RESOURCE_SERVER_SSH,
  [ResourceOption.RESOURCE_DATABASES]: ClusterResource.RESOURCE_DATABASES,
  [ResourceOption.RESOURCE_KUBERNETES]: ClusterResource.RESOURCE_KUBERNETES,
  [ResourceOption.RESOURCE_WEB_APPLICATIONS]:
    ClusterResource.RESOURCE_WEB_APPLICATIONS,
};
