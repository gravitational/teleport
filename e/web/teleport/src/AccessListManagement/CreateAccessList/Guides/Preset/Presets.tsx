import { BaseView } from 'teleport/components/Wizard/flow';

import { Finished } from '../../Finished';
import { NavView } from '../../types';
import { BasicInformation } from './BasicInformation/BasicInformation';
import { DefineMembership } from './DefineUser/DefineMembership';
import { DefineOwnership } from './DefineUser/DefineOwnership';

export const presetGuideViews: BaseView<NavView>[] = [
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
