import { BaseView } from 'teleport/components/Wizard/flow';
import { AccessListEvent } from 'teleport/services/userEvent/accessListEvents';

import { NavView } from '../../types';

export type AccessListView = BaseView<
  NavView & {
    /**
     * If undefined, no usage event will be emitted for this view.
     */
    eventName?: AccessListEvent;
    isFinishedStep?: boolean;
  }
>;
