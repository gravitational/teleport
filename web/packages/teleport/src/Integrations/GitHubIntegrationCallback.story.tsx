/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { GitHubIntegrationCallbackView } from './GitHubIntegrationCallback';

export default {
  title: 'Teleport/Integrations/GitHubIntegrationCallback',
};

export const Processing = () => (
  <GitHubIntegrationCallbackView processing={true} error="" />
);

export const Success = () => (
  <GitHubIntegrationCallbackView processing={false} error="" />
);

export const Failed = () => (
  <GitHubIntegrationCallbackView
    processing={false}
    error='Session user "bob" does not match auth request user "alice".'
  />
);

export const MissingParams = () => (
  <GitHubIntegrationCallbackView
    processing={false}
    error="Missing required parameters from GitHub."
  />
);
