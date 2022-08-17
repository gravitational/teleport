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
import React, { useEffect, useState } from 'react';
import { Cloud } from 'design/Icon';
import SlideTabs from 'design/SlideTabs';
import styled from 'styled-components';
import { useLocation } from 'react-router';

import { Image, Text, Box, Flex } from 'design';

import AddApp from 'teleport/Apps/AddApp';
import AddDatabase from 'teleport/Databases/AddDatabase';
import AddKube from 'teleport/Kubes/AddKube';
import useTeleport from 'teleport/useTeleport';

import { ActionButtons } from '../Shared';

import applicationIcon from './assets/application.png';
import databaseIcon from './assets/database.png';
import serverIcon from './assets/server.png';
import k8sIcon from './assets/kubernetes.png';

import type { TabComponent } from 'design/SlideTabs/SlideTabs';
import type { ResourceType, ResourceLocation } from '../resource-lists';
import type { AgentStepProps } from '../types';
import type { State } from '../useDiscover';
import type { AuthType } from 'teleport/services/user';

export default function Container(props: AgentStepProps) {
  const ctx = useTeleport();
  const ctxState = ctx.storeUser.state;
  return (
    <SelectResource
      authType={ctxState.authType}
      isEnterprise={ctx.isEnterprise}
      nextStep={props.nextStep}
      username={ctxState.username}
      version={ctxState.cluster.authVersion}
    />
  );
}

type ValidResourceTypes =
  | 'application'
  | 'database'
  | 'desktop'
  | 'kubernetes'
  | 'server';

type Loc = {
  state: {
    entity: ValidResourceTypes;
  };
};

type Props = {
  authType: AuthType;
  isEnterprise: boolean;
  nextStep: State['nextStep'];
  username: string;
  version: string;
};

export function SelectResource({
  authType,
  isEnterprise,
  nextStep,
  username,
  version,
}: Props) {
  const location: Loc = useLocation();

  const [selectedResource, setSelectedResource] = useState<ValidResourceTypes>(
    location?.state?.entity
  );
  const [selectedType, setSelectedType] = useState('');
  const [disableProceed, setDisableProceed] = useState<boolean>(true);
  const [showAddApp, setShowAddApp] = useState(false);
  const [showAddKube, setShowAddKube] = useState(false);
  const [showAddDB, setShowAddDB] = useState(false);

  const tabs: TabComponent[] = [
    {
      name: 'server',
      component: (
        <Flex style={{ lineHeight: '31px' }}>
          <Image src={serverIcon} width="32px" mr={2} /> Server
        </Flex>
      ),
    },
    {
      name: 'database',
      component: (
        <>
          <Flex style={{ lineHeight: '31px' }}>
            <Image src={databaseIcon} width="32px" mr={2} /> Database
          </Flex>
        </>
      ),
    },

    {
      name: 'kubernetes',
      component: (
        <Flex style={{ lineHeight: '31px' }}>
          <Image src={k8sIcon} width="32px" mr={2} /> Kubernetes
        </Flex>
      ),
    },

    {
      name: 'application',
      component: (
        <Flex style={{ lineHeight: '31px' }}>
          <Image src={applicationIcon} width="32px" mr={2} /> Application
        </Flex>
      ),
    },

    {
      name: 'desktop',
      component: (
        <Flex style={{ lineHeight: '31px' }}>
          <Image src={serverIcon} width="32px" mr={2} /> Desktop
        </Flex>
      ),
    },
  ];

  useEffect(() => {
    if (selectedResource === 'server') {
      // server doesn't have any additional deployment options
      setDisableProceed(false);
      return;
    }
    if (selectedResource && selectedType) {
      setDisableProceed(false);
      return;
    }
    setDisableProceed(true);
  }, [selectedResource, selectedType]);

  const initialSelected = tabs.findIndex(
    component => component.name === location?.state?.entity
  );

  return (
    <Box width="1020px">
      <Text typography="h4">Resource Selection</Text>
      <Text mb={4}>
        Users are able to add and access many different types of resources
        through Teleport. Start by selecting the type of resource you want to
        add.
      </Text>
      <Text mb={2}>Select Resource Type</Text>
      <SlideTabs
        initialSelected={initialSelected > 0 ? initialSelected : 0}
        tabs={tabs}
        onChange={index =>
          setSelectedResource(tabs[index].name as ValidResourceTypes)
        }
      />
      {selectedResource === 'database' && (
        // As we're focusing on the server flow uncomment this when we start
        // implementing the database support.
        // <SelectDBDeploymentType
        //   selectedType={selectedType}
        //   setSelectedType={setSelectedType}
        //   resourceTypes={resourceTypes}
        // />
        <ActionButtons
          onProceed={() => {
            setShowAddDB(true);
          }}
          disableProceed={false}
        />
      )}
      {selectedResource === 'application' && (
        <ActionButtons
          onProceed={() => {
            setShowAddApp(true);
          }}
          disableProceed={false}
        />
      )}
      {selectedResource === 'desktop' && (
        <ActionButtons
          proceedHref="https://goteleport.com/docs/desktop-access/getting-started/"
          disableProceed={false}
        />
      )}
      {selectedResource === 'kubernetes' && (
        <ActionButtons
          onProceed={() => {
            setShowAddKube(true);
          }}
          disableProceed={false}
        />
      )}
      {selectedResource === 'server' && (
        <ActionButtons
          onProceed={() => {
            nextStep();
          }}
          disableProceed={disableProceed}
        />
      )}
      {showAddApp && <AddApp onClose={() => setShowAddApp(false)} />}
      {showAddKube && <AddKube onClose={() => setShowAddKube(false)} />}
      {showAddDB && (
        <AddDatabase
          isEnterprise={isEnterprise}
          username={username}
          version={version}
          authType={authType}
          onClose={() => setShowAddDB(false)}
        />
      )}
    </Box>
  );
}

type SelectResourceProps = {
  onSelect: (string) => void;
};

function SelectDBDeploymentType({
  selectedType,
  setSelectedType,
  resourceTypes,
}: SelectDBDeploymentTypeProps) {
  type FilterType = 'All' | ResourceLocation;
  const filterTabs: FilterType[] = ['All', 'AWS', 'Self-Hosted'];
  const [filter, setFilter] = useState<FilterType>('All');
  return (
    <Box mt={6}>
      <Flex alignItems="center" justifyContent="space-between">
        <Text mb={2}>Select Deployment Type</Text>
        <Box width="379px">
          <SlideTabs
            appearance="round"
            size="medium"
            tabs={filterTabs}
            onChange={index => setFilter(filterTabs[index])}
          />
        </Box>
      </Flex>
      <Flex
        flexWrap="wrap"
        mt={4}
        justifyContent="space-between"
        gap="12px 12px"
        rowGap="15px"
      >
        {resourceTypes
          .filter(resource => filter === 'All' || resource.type === filter)
          .map(resource => (
            <ResourceTypeOption
              onClick={() => setSelectedType(resource.key)}
              key={resource.key}
              selected={selectedType === resource.key}
            >
              <Flex justifyContent="space-between" mb={2}>
                <Cloud />
                <Tag>popular</Tag>
              </Flex>
              {resource.name}
            </ResourceTypeOption>
          ))}
      </Flex>
    </Box>
  );
}

type SelectDBDeploymentTypeProps = {
  selectedType: string;
  setSelectedType: (string) => void;
  resourceTypes: ResourceType[];
};

const ResourceTypeOption = styled.div`
  background: rgba(255, 255, 255, 0.05);
  border: ${props =>
    !props.selected
      ? '2px solid rgba(255, 255, 255, 0)'
      : '2px solid rgba(255, 255, 255, 0.1);'};
  border-radius: 8px;
  box-sizing: border-box;
  cursor: pointer;
  height: 72px;
  padding: 12px;
  width: 242px;

  &:hover {
    border: 2px solid rgba(255, 255, 255, 0.1);
  }
`;

const Tag = styled.div`
  align-items: center;
  background-color: #512fc9;
  border-radius: 33px;
  box-sizing: border-box;
  font-size: 10px;
  height: 15px;
  line-height: 11px;
  padding: 2px 10px;
  max-width: 57px;
`;
