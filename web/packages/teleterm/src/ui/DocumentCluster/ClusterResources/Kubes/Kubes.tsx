/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react';
import Table, {
  Cell,
  ClickableLabelCell,
  StyledTableWrapper,
} from 'design/DataTable';
import { ButtonBorder } from 'design';
import { Danger } from 'design/Alert';
import { SearchPanel, SearchPagination } from 'shared/components/Search';

import { makeKube } from 'teleterm/ui/services/clusters';

import { DarkenWhileDisabled } from '../DarkenWhileDisabled';
import { getEmptyTableText } from '../getEmptyTableText';

import { useKubes, State } from './useKubes';

export default function Container() {
  const state = useKubes();
  return <KubeList {...state} />;
}

function KubeList(props: State) {
  const {
    connect,
    fetchAttempt,
    agentFilter,
    pageCount,
    customSort,
    prevPage,
    nextPage,
    updateQuery,
    onAgentLabelClick,
    updateSearch,
  } = props;
  const kubes = fetchAttempt.data?.agentsList.map(makeKube) || [];
  const disabled = fetchAttempt.status === 'processing';
  const emptyText = getEmptyTableText(fetchAttempt.status, 'kubes');

  return (
    <>
      {fetchAttempt.status === 'error' && (
        <Danger>{fetchAttempt.statusText}</Danger>
      )}
      <StyledTableWrapper borderRadius={3}>
        <SearchPanel
          updateQuery={updateQuery}
          updateSearch={updateSearch}
          pageIndicators={pageCount}
          filter={agentFilter}
          showSearchBar={true}
          disableSearch={disabled}
        />
        <DarkenWhileDisabled disabled={disabled}>
          <Table
            data={kubes}
            columns={[
              {
                key: 'name',
                headerText: 'Name',
                isSortable: true,
              },
              {
                key: 'labels',
                headerText: 'Labels',
                render: ({ labels }) => (
                  <ClickableLabelCell
                    labels={labels}
                    onClick={onAgentLabelClick}
                  />
                ),
              },
              {
                altKey: 'connect-btn',
                render: kube => renderConnectButtonCell(kube.uri, connect),
              },
            ]}
            customSort={customSort}
            emptyText={emptyText}
          />
          <SearchPagination prevPage={prevPage} nextPage={nextPage} />
        </DarkenWhileDisabled>
      </StyledTableWrapper>
    </>
  );
}

export const renderConnectButtonCell = (
  uri: string,
  connect: (kubeUri: string) => void
) => {
  return (
    <Cell align="right">
      <ButtonBorder
        size="small"
        onClick={() => {
          connect(uri);
        }}
      >
        Connect
      </ButtonBorder>
    </Cell>
  );
};
