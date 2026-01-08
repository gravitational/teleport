import { BaseView } from 'teleport/components/Wizard/flow';

import { Finished } from '../../Finished';
import { NavView } from '../../types';
import { AccessListForm } from './AccessListForm';

export const customViews: BaseView<NavView>[] = [
  {
    title: 'Form',
    Component: <AccessListForm />,
  },
  {
    title: 'Finished',
    Component: <Finished />,
  },
];
