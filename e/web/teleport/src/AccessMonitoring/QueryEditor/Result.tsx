import React, { PropsWithChildren, useEffect, useState } from 'react';
import styled from 'styled-components';

import useStickyClusterId from 'teleport/useStickyClusterId';
import { useAttemptNext } from 'shared/hooks';
import Indicator from 'design/Indicator';
import { Flex } from 'design';

import { getQueryResult } from 'e-teleport/AccessMonitoring/service';
import { QueryResult } from 'e-teleport/AccessMonitoring/types';

interface ResultProps {
  resultId: string;
}

const Table = styled.table`
  width: 100%;
  border-collapse: collapse;
`;

const TableHeader = styled.th`
  text-align: left;

  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;

  svg {
    opacity: 0.5;
  }
`;

const TableHead = styled.thead`
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

const TableData = styled.td`
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
`;

const Container = styled.div`
  overflow-x: auto;
  margin-top: ${p => p.theme.space[5]}px;
  background: ${p => p.theme.colors.levels.popout};
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 7px;
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

  return (
    <Container>
      <Table>
        <TableHead>
          {header.data.map((item, index) => (
            <TableHeader key={index}>{item}</TableHeader>
          ))}
        </TableHead>

        <tbody>
          {rest.map((row, i) => (
            <tr key={i}>
              {row.data.map((item, index) => (
                <TableData key={index}>{item}</TableData>
              ))}
            </tr>
          ))}
        </tbody>
      </Table>
    </Container>
  );
}
