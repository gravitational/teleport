import { AccessListEvent } from 'teleport/services/userEvent/accessListEvents';

import { Finished } from '../../Finished';
import { AccessListView } from '../Preset/view';
import { AccessListForm } from './AccessListForm';

export const customViews: AccessListView[] = [
  {
    title: 'Form',
    Component: <AccessListForm />,
    eventName: AccessListEvent.Custom,
  },
  {
    title: 'Finished',
    Component: <Finished />,
    isFinishedStep: true,
  },
];
