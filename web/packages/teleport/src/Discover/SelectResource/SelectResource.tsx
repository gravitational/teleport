/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
import React, { useState } from 'react';
import SlideTabs from 'design/SlideTabs';
import { Image, Text, Box, Flex } from 'design';

import AddApp from 'teleport/Apps/AddApp';
import AddDatabase from 'teleport/Databases/AddDatabase';
import AddKube from 'teleport/Kubes/AddKube';
import useTeleport from 'teleport/useTeleport';

import { Acl } from 'teleport/services/user';

import { Header, HeaderSubtitle } from '../Shared';

import applicationIcon from './assets/application.png';
import databaseIcon from './assets/database.png';
import serverIcon from './assets/server.png';
import k8sIcon from './assets/kubernetes.png';

import { ApplicationResource } from './ApplicationResource';
import { DatabaseResource } from './DatabaseResource';
import { DesktopResource } from './DesktopResource';
import { KubernetesResource } from './KubernetesResource';
import { ServerResource } from './ServerResource';

import type { UserContext } from 'teleport/services/user';
import type { State, AgentKind } from '../useDiscover';
import type { AgentStepProps } from '../types';
import type { TabComponent } from 'design/SlideTabs/SlideTabs';

export default function Container(props: AgentStepProps) {
  const ctx = useTeleport();
  const userContext = ctx.storeUser.state;

  return (
    <SelectResource
      userContext={userContext}
      isEnterprise={ctx.isEnterprise}
      nextStep={props.nextStep}
      selectedResource={props.selectedAgentKind}
      onSelectResource={props.onSelectResource}
    />
  );
}

function checkPermissions(acl: Acl, tab: Tab) {
  const basePermissionsNeeded = [acl.tokens.create];

  const permissionsNeeded = [
    ...basePermissionsNeeded,
    ...tab.permissionsNeeded,
  ];

  // if some (1+) are false, we do not have enough permissions
  return permissionsNeeded.some(value => !value);
}

type Props = {
  userContext: UserContext;
  isEnterprise: boolean;
  nextStep: State['nextStep'];
  selectedResource: State['selectedAgentKind'];
  onSelectResource: State['onSelectResource'];
};

interface Tab extends TabComponent {
  permissionsNeeded: boolean[];
}

export function SelectResource({
  isEnterprise,
  nextStep,
  userContext,
  selectedResource,
  onSelectResource,
}: Props) {
  const { acl } = userContext;

  const [showAddApp, setShowAddApp] = useState(false);
  const [showAddKube, setShowAddKube] = useState(false);
  const [showAddDB, setShowAddDB] = useState(false);

  const tabs: Tab[] = [
    {
      name: 'server',
      component: <TabItem iconSrc={serverIcon} title="Server" />,
      permissionsNeeded: [acl.nodes.list],
    },

    {
      name: 'database',
      component: <TabItem iconSrc={databaseIcon} title="Database" />,
      permissionsNeeded: [acl.dbServers.read, acl.dbServers.list],
    },

    {
      name: 'kubernetes',
      component: <TabItem iconSrc={k8sIcon} title="Kubernetes" />,
      permissionsNeeded: [acl.kubeServers.read, acl.kubeServers.list],
    },

    {
      name: 'application',
      component: <TabItem iconSrc={applicationIcon} title="Application" />,
      permissionsNeeded: [acl.appServers.read, acl.appServers.list],
    },

    {
      name: 'desktop',
      component: <TabItem iconSrc={serverIcon} title="Desktop" />,
      permissionsNeeded: [acl.desktops.read, acl.desktops.list],
    },
  ];

  const index = tabs.findIndex(
    component => component.name === selectedResource
  );
  const selectedTabIndex = Math.max(0, index);

  const disabled = checkPermissions(acl, tabs[selectedTabIndex]);

  return (
    <Box>
      <Header>Select Resource Type</Header>
      <HeaderSubtitle>
        Users are able to add and access many different types of resources
        through Teleport. <br />
        Start by selecting the type of resource you want to add.
      </HeaderSubtitle>
      <SlideTabs
        initialSelected={selectedTabIndex}
        tabs={tabs}
        onChange={index => onSelectResource(tabs[index].name as AgentKind)}
      />
      {selectedResource === 'database' && (
        <DatabaseResource
          disabled={disabled}
          onProceed={() => setShowAddDB(true)}
        />
      )}
      {selectedResource === 'application' && (
        <ApplicationResource
          disabled={disabled}
          onProceed={() => setShowAddApp(true)}
        />
      )}
      {selectedResource === 'desktop' && (
        <DesktopResource disabled={disabled} />
      )}
      {selectedResource === 'kubernetes' && (
        <KubernetesResource
          disabled={disabled}
          onProceed={() => setShowAddKube(true)}
        />
      )}
      {selectedResource === 'server' && (
        <ServerResource disabled={disabled} onProceed={nextStep} />
      )}
      {showAddApp && <AddApp onClose={() => setShowAddApp(false)} />}
      {showAddKube && <AddKube onClose={() => setShowAddKube(false)} />}
      {showAddDB && (
        <AddDatabase
          isEnterprise={isEnterprise}
          username={userContext.username}
          version={userContext.cluster.authVersion}
          authType={userContext.authType}
          onClose={() => setShowAddDB(false)}
        />
      )}
    </Box>
  );
}

const TabItem = ({ iconSrc, title }: { iconSrc: string; title: string }) => (
  <Flex
    css={`
      align-items: center;
    `}
  >
    <Image src={iconSrc} width="32px" mr={2} />
    <Text bold typography="h5">
      {title}
    </Text>
  </Flex>
);
