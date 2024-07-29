/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import React, { useEffect } from 'react';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { Timeframe } from './Timeframe';

const ctx = createTeleportContext();

const defaultIsEnterprise = cfg.isEnterprise;
const defaultAccessMonitoringEntitlement = cfg.entitlements.AccessMonitoring;

export default {
  title: 'TeleportE/AccessMonitoring',
  decorators: [
    Story => {
      useEffect(() => {
        cfg.isEnterprise = true;
        // Clean up
        return () => {
          cfg.isEnterprise = defaultIsEnterprise;
          cfg.entitlements.AccessMonitoring =
            defaultAccessMonitoringEntitlement;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const TimeframeDropdownWithLimitAndCta = () => {
  cfg.entitlements.AccessMonitoring = { enabled: true, limit: 30 };
  return (
    <ContextProvider ctx={ctx}>
      <Timeframe onChange={() => null} days={30} />
    </ContextProvider>
  );
};

export const TimeframeDropdownUnlimited = () => {
  cfg.entitlements.AccessMonitoring = { enabled: true, limit: 0 };
  return (
    <ContextProvider ctx={ctx}>
      <Timeframe onChange={() => null} days={30} />
    </ContextProvider>
  );
};
