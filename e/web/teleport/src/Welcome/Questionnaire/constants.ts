import { ResourceIconName } from 'design/ResourceIcon';
import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';
import { Option } from 'shared/components/Select';

import {
  EmployeeOption,
  ResourceOption,
  TeamOption,
  TitleOption,
} from './types';

export type EmployeeSelectOption = Option<EmployeeOption, EmployeeOption>;
export const EmployeeSelectOptions: EmployeeSelectOption[] = Object.keys(
  EmployeeOption
).map(key => ({
  value: EmployeeOption[key],
  label: EmployeeOption[key],
}));

export type TeamSelectOptionValue = keyof typeof TeamOption;
export type TeamSelectOption = Option<TeamSelectOptionValue, TeamOption>;
export const teamSelectOptions: TeamSelectOption[] = (
  Object.keys(TeamOption) as Array<TeamSelectOptionValue>
).map(key => ({
  value: key,
  label: TeamOption[key],
}));

export type TitleSelectOptionValue = keyof typeof TitleOption;
export type TitleSelectOption = Option<TitleSelectOptionValue, TitleOption>;
export const titleSelectOptions: TitleSelectOption[] = (
  Object.keys(TitleOption) as Array<TitleSelectOptionValue>
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

export const GetResourceIcon = (key: ResourceOption): ResourceIconName => {
  switch (key) {
    case ResourceOption.RESOURCE_WEB_APPLICATIONS:
      return 'application';
    case ResourceOption.RESOURCE_WINDOWS_DESKTOPS:
      return 'windows';
    case ResourceOption.RESOURCE_SERVER_SSH:
      return 'server';
    case ResourceOption.RESOURCE_DATABASES:
      return 'database';
    case ResourceOption.RESOURCE_KUBERNETES:
      return 'kubeserver';
    default:
      key satisfies never;
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
