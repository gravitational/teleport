import api from 'teleport/services/api';
import cfg from 'teleport/config';

import { UserEvent, PreUserEvent, DiscoverEventRequest } from './types';

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

  captureDiscoverEvent(event: DiscoverEventRequest) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.captureUserEventPath, {
      method: 'POST',
      body: JSON.stringify(event),
    });
  },
};
