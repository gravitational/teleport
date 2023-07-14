/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import {
  UserEvent,
  PreUserEvent,
  DiscoverEventRequest,
  CtaEvent,
  CaptureEvent,
  IntegrationEnrollEventRequest,
} from './types';

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

  captureIntegrationEnrollEvent(event: IntegrationEnrollEventRequest) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.captureUserEventPath, {
      method: 'POST',
      body: JSON.stringify(event),
    });
  },

  captureCtaEvent(event: CtaEvent) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.captureUserEventPath, {
      method: 'POST',
      body: JSON.stringify({
        event: CaptureEvent.UiCallToActionClickEvent,
        eventData: event,
      }),
    });
  },
};
