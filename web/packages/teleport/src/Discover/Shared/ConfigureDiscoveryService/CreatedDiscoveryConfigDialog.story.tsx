/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { CreatedDiscoveryConfigDialog } from './CreatedDiscoveryConfigDialog';

export default {
  title: 'Teleport/Discover/Shared/ConfigureDiscoveryService/CreatedDialog',
};

export const Success = () => (
  <CreatedDiscoveryConfigDialog
    retry={() => null}
    close={() => null}
    next={() => null}
    region="us-east-1"
    notifyAboutDelay={false}
    attempt={{ status: 'success' }}
  />
);

export const SuccessWithDelay = () => (
  <CreatedDiscoveryConfigDialog
    retry={() => null}
    close={() => null}
    next={() => null}
    region="us-east-1"
    notifyAboutDelay={true}
    attempt={{ status: 'success' }}
  />
);

export const Loading = () => (
  <CreatedDiscoveryConfigDialog
    retry={() => null}
    close={() => null}
    next={() => null}
    region="us-east-1"
    notifyAboutDelay={false}
    attempt={{ status: 'processing' }}
  />
);

export const Failed = () => (
  <CreatedDiscoveryConfigDialog
    retry={() => null}
    close={() => null}
    next={() => null}
    region="us-east-1"
    notifyAboutDelay={false}
    attempt={{ status: 'failed', statusText: 'some kind of error message' }}
  />
);
