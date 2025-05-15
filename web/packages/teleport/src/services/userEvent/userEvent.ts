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
  CreateNewRoleSaveClickEvent,
  CreateNewRoleSaveClickEventData,
  CtaEvent,
  CtaEventRequest,
  DiscoverEventRequest,
  FeatureRecommendationEvent,
  FeatureRecommendationEventRequest,
  IntegrationEnrollEventRequest,
  PreUserEvent,
  UserEvent,
} from './types';

function captureEvent({
  event,
  path = cfg.api.captureUserEventPath,
}: {
  event:
    | UserEvent
    | PreUserEvent
    | DiscoverEventRequest
    | IntegrationEnrollEventRequest
    | CtaEventRequest
    | FeatureRecommendationEventRequest
    | CreateNewRoleSaveClickEvent;
  path?: string;
}) {
  // using api.fetch instead of api.fetchJSON
  // because we are not expecting a JSON response
  void api.fetch(path, {
    method: 'POST',
    body: JSON.stringify(event),
  });
}

export const userEventService = {
  captureUserEvent(event: UserEvent) {
    captureEvent({ event });
  },

  capturePreUserEvent(event: PreUserEvent) {
    captureEvent({ event, path: cfg.api.capturePreUserEventPath });
  },

  captureDiscoverEvent(event: DiscoverEventRequest) {
    captureEvent({ event });
  },

  captureIntegrationEnrollEvent(event: IntegrationEnrollEventRequest) {
    captureEvent({ event });
  },

  captureCtaEvent(eventData: CtaEvent) {
    captureEvent({
      event: {
        event: CaptureEvent.UiCallToActionClickEvent,
        eventData,
      },
    });
  },

  captureFeatureRecommendationEvent(eventData: FeatureRecommendationEvent) {
    captureEvent({
      event: {
        event: CaptureEvent.FeatureRecommendationEvent,
        eventData,
      },
    });
  },

  captureCreateNewRoleSaveClickEvent(
    eventData: CreateNewRoleSaveClickEventData
  ) {
    captureEvent({
      event: {
        event: CaptureEvent.CreateNewRoleSaveClickEvent,
        eventData,
      },
    });
  },
};
