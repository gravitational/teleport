import { DefineAccess } from 'e-teleport/AccessListManagement/GuideEditor/Preset/DefineAccess/DefineAccess';
import { DefineIdentities } from 'e-teleport/AccessListManagement/GuideEditor/Preset/DefineIdentities/DefineIdentities';
import { AccessListEvent } from 'teleport/services/userEvent/accessListEvents';

import { DeploymentMethods } from '../../DeploymentMethods/DeploymentMethods';
import { BasicInformation } from './BasicInformation/BasicInformation';
import { DefineMembership } from './DefineUser/DefineMembership';
import { DefineOwnership } from './DefineUser/DefineOwnership';
import { AccessListView } from './view';

export const presetGuideViews: AccessListView[] = [
  {
    title: 'Define Access to Resources',
    Component: <DefineAccess />,
    eventName: AccessListEvent.DefineAccess,
  },
  {
    title: 'Define Resource Identities or Principals',
    Component: <DefineIdentities />,
    eventName: AccessListEvent.DefineIdentities,
  },
  {
    title: 'Basic Information',
    Component: <BasicInformation />,
    eventName: AccessListEvent.DefineBasicInfo,
  },
  {
    title: 'Define Membership',
    Component: <DefineMembership />,
    eventName: AccessListEvent.DefineMembers,
  },
  {
    title: 'Define Ownership',
    Component: <DefineOwnership />,
    eventName: AccessListEvent.DefineOwners,
  },
  {
    title: 'Deployment',
    Component: <DeploymentMethods />,
    isFinishedStep: true,
    eventName: AccessListEvent.Completed,
  },
];
