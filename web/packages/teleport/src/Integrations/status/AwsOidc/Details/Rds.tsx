/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useEffect, useRef } from 'react';
import { useLocation, useParams } from 'react-router';

import { TabBorder, TabContainer, TabsContainer } from 'design/Tabs/Tabs';

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import { Agents } from 'teleport/Integrations/status/AwsOidc/Details/Agents';
import { Rules } from 'teleport/Integrations/status/AwsOidc/Details/Rules';
import { IntegrationKind } from 'teleport/services/integrations';

export enum RdsTab {
  Agents = 'agents',
  Rules = 'rules',
}

export function Rds() {
  const { type, name, resourceKind } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind: AwsResource;
  }>();

  const { search } = useLocation();
  const searchParams = new URLSearchParams(search);
  const tab = (searchParams.get('tab') as RdsTab) || RdsTab.Rules;

  const borderRef = useRef<HTMLDivElement>(null);
  const parentRef = useRef<HTMLDivElement>(null);

  // todo (michellescripts) the following implementation mimics the implementation of tabs in
  //  e/web/teleport/src/AccessMonitoring/AccessMonitoring.tsx which is refactored/moved into a shared
  //  design component, web/packages/design/src/Tabs/Tabs.ts. When refactoring AccessMonitoring to use the shared
  //  component, consider updating both instances logic to be plain css
  useEffect(() => {
    if (!parentRef.current || !borderRef.current) {
      return;
    }

    const activeElement = parentRef.current.querySelector(
      `[data-tab-id="${tab}"]`
    );

    if (activeElement) {
      const parentBounds = parentRef.current.getBoundingClientRect();
      const activeBounds = activeElement.getBoundingClientRect();

      const left = activeBounds.left - parentBounds.left;
      const width = activeBounds.width;

      borderRef.current.style.left = `${left}px`;
      borderRef.current.style.width = `${width}px`;
    }
  }, [tab]);

  return (
    <>
      <TabsContainer ref={parentRef}>
        <TabContainer
          data-tab-id={RdsTab.Rules}
          selected={tab === RdsTab.Rules}
          to={`${cfg.getIntegrationStatusResourcesRoute(
            type,
            name,
            resourceKind
          )}?tab=${RdsTab.Rules}`}
        >
          Enrollment Rules
        </TabContainer>
        <TabContainer
          data-tab-id={RdsTab.Agents}
          selected={tab === RdsTab.Agents}
          to={`${cfg.getIntegrationStatusResourcesRoute(
            type,
            name,
            resourceKind
          )}?tab=${RdsTab.Agents}`}
        >
          Agents
        </TabContainer>
        <TabBorder ref={borderRef} />
      </TabsContainer>
      {tab === RdsTab.Rules && <Rules />}
      {tab === RdsTab.Agents && <Agents />}
    </>
  );
}
