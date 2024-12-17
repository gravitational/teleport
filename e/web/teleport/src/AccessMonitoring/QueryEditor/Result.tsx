import { PropsWithChildren, useEffect, useState } from 'react';
import styled from 'styled-components';

import Table from 'design/DataTable';

import useStickyClusterId from 'teleport/useStickyClusterId';
import { useAttemptNext } from 'shared/hooks';
import Indicator from 'design/Indicator';
import { Flex } from 'design';

import { getQueryResult } from 'e-teleport/AccessMonitoring/service';

import DownloadButton from 'e-teleport/AccessMonitoring/QueryEditor/DownloadButton';

import type { QueryResult } from 'e-teleport/AccessMonitoring/types';

interface ResultProps {
  resultId: string;
}

const TableContainer = styled.div`
  position: relative;
  width: 100%;
  max-height: 500px;
  overflow-x: auto;
  overflow-y: auto;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: ${p => p.theme.radii[3]}px;
`;

const Container = styled.div`
  display: flex;
  flex-direction: column;
  row-gap: ${p => p.theme.space[3]}px;
  margin-top: ${p => p.theme.space[5]}px;
  background: ${p => p.theme.colors.levels.popout};
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p => p.theme.space[3]}px;
`;

const DetailRow = styled.div`
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  font-weight: bold;
  font-size: 14px;
  padding-left: ${p => p.theme.space[2]}px;
`;

const ResultsTable = styled(Table)`
  width: 100%;
  position: relative;
  background: ${p => p.theme.colors.levels.elevated};

  thead {
    position: sticky;
    top: 0;
    z-index: 2;
    background: ${p => p.theme.colors.levels.surface};
    border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};

    tr {
      box-shadow: 0 0 0 1px ${p => p.theme.colors.spotBackground[1]};

      th {
        padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;

        &:not(:last-child) {
          box-shadow: 1px 0 0 0 ${p => p.theme.colors.spotBackground[1]};
        }
      }
    }
  }

  tbody {
    tr {
      border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};

      td {
        &:not(:last-child) {
          border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
        }
      }
    }
  }
`;

function EmptyWrapper(props: PropsWithChildren<unknown>) {
  return (
    <Flex alignItems="center" justifyContent="center" m={4}>
      {props.children}
    </Flex>
  );
}

export function Result(props: ResultProps) {
  const { clusterId } = useStickyClusterId();

  const { attempt, run } = useAttemptNext('processing');

  const [result, setResult] = useState<QueryResult | null>(null);

  useEffect(() => {
    async function init() {
      const res = await getQueryResult(clusterId, props.resultId);
      setResult(res.result);
    }

    run(init);
  }, [props.resultId]);

  if (attempt.status === 'processing') {
    return (
      <EmptyWrapper>
        <Indicator />
      </EmptyWrapper>
    );
  }

  if (attempt.status === 'failed') {
    return <EmptyWrapper>{attempt.statusText}</EmptyWrapper>;
  }

  const [header, ...rest] = result.rows;

  if (rest.length === 0) {
    return <EmptyWrapper>No results</EmptyWrapper>;
  }

  const tableData = rest.map(({ data }, idx) =>
    data.reduce(
      (acc, item, idx) => ({ ...acc, [header.data[idx].toLowerCase()]: item }),
      { NUM_SORT_COL: idx + 1 } as Record<string, string | number>
    )
  );

  const tableColumns = header.data.reduce(
    (acc, item) => [
      ...acc,
      {
        key: item.toLowerCase(),
        headerText: item,
        isSortable: false,
      },
    ],
    [
      {
        key: 'NUM_SORT_COL',
        headerText: '#',
        isSortable: false,
        isNonRender: true,
      },
    ]
  );

  return (
    <Container>
      <DetailRow>
        <span>
          {rest.length} result
          {rest.length === 1 ? '' : 's'}
        </span>
        <DownloadButton
          resultId={props.resultId}
          header={header.data}
          rows={rest.map(({ data }) => data)}
        />
      </DetailRow>

      <TableContainer>
        {/* @ts-expect-error - Returned keys are unknown */}
        <ResultsTable
          data={tableData}
          columns={tableColumns}
          emptyText="No results"
          initialSort={{ key: 'NUM_SORT_COL', dir: 'ASC' }}
        />
      </TableContainer>
    </Container>
  );
}
