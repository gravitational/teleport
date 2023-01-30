import api from 'teleport/services/api';
import cfg from 'teleport/config';

import { CaptureEvent } from './types';

export type UserEvent = {
  event: CaptureEvent;
  alert?: string;
};

export type PreUserEvent = UserEvent & {
  username: string;
  mfaType?: string;
  loginFlow?: string;
};

export const userEventService = {
  captureUserEvent(userEvent: UserEvent) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.captureUserEventPath, {
      method: 'POST',
      body: JSON.stringify(userEvent),
    });
  },

  capturePreUserEvent(preUserEvent: PreUserEvent) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.capturePreUserEventPath, {
      method: 'POST',
      body: JSON.stringify({ ...preUserEvent }),
    });
  },
};
