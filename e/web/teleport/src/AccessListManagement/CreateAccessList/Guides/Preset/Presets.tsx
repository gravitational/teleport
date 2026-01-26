import { DefineAccess } from 'e-teleport/AccessListManagement/GuideEditor/Preset/DefineAccess/DefineAccess';
import { DefineIdentities } from 'e-teleport/AccessListManagement/GuideEditor/Preset/DefineIdentities/DefineIdentities';
import { BaseView } from 'teleport/components/Wizard/flow';

import { Finished } from '../../Finished';
import { NavView } from '../../types';
import { BasicInformation } from './BasicInformation/BasicInformation';
import { DefineMembership } from './DefineUser/DefineMembership';
import { DefineOwnership } from './DefineUser/DefineOwnership';

export const presetGuideViews: BaseView<NavView>[] = [
  {
    title: 'Define Access to Resources',
    Component: <DefineAccess />,
  },
  {
    title: 'Define Resource Identities or Principals',
    Component: <DefineIdentities />,
  },
  {
    title: 'Basic Information',
    Component: <BasicInformation />,
  },
  {
    title: 'Define Membership',
    Component: <DefineMembership />,
  },
  {
    title: 'Define Ownership',
    Component: <DefineOwnership />,
  },
  {
    title: 'Finished',
    Component: <Finished />,
  },
];
