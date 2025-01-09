/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import {
  CaptureEvent,
  CtaEvent,
  DiscoverEventRequest,
  FeatureRecommendationEvent,
  IntegrationEnrollEventRequest,
  PreUserEvent,
  UserEvent,
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

  captureFeatureRecommendationEvent(event: FeatureRecommendationEvent) {
    // using api.fetch instead of api.fetchJSON
    // because we are not expecting a JSON response
    void api.fetch(cfg.api.captureUserEventPath, {
      method: 'POST',
      body: JSON.stringify({
        event: CaptureEvent.FeatureRecommendationEvent,
        eventData: event,
      }),
    });
  },
};
