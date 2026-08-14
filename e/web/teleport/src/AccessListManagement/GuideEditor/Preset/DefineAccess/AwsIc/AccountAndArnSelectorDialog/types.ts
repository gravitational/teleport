import { UnifiedResourceApp } from 'shared/components/UnifiedResources';

import { PermissionSet } from 'teleport/services/apps';

export type PermissionOption = {
  value: PermissionSet;
  label: string;
  selected: boolean;
};

export type AccountOption = {
  value: UnifiedResourceApp;
  label: string;
  selected: boolean;
};
