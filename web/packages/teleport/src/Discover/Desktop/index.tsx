import React from 'react';

import { ResourceViewConfig } from 'teleport/Discover/flow';
import { ResourceSpec } from 'teleport/Discover/SelectResource';
import { ResourceKind } from 'teleport/Discover/Shared';
import SetupWDS from './SetupWDS/SetupWDS';
import ConnectWindowsDesktop from './ConnectWindowsDesktop/ConnectWindowsDesktop';
import { SetupAccess } from './SetupAccess';
import { DesktopWrapper } from './DesktopWrapper';

export const DesktopResource: ResourceViewConfig<ResourceSpec> = {
  kind: ResourceKind.Desktop,
  wrapper: (component: React.ReactNode) => (
    <DesktopWrapper>{component}</DesktopWrapper>
  ),
  views: [
    {
      title: 'Setup Windows Desktop Service',
      component: SetupWDS,
    },
    {
      title: 'Setup Access',
      component: SetupAccess,
    },
    {
      title: 'Add Windows Desktops',
      component: ConnectWindowsDesktop,
    }
  ]
}