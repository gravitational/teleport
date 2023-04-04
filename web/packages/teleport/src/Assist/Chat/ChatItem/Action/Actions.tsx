import React, {
  Children,
  PropsWithChildren,
  ReactNode,
  useCallback,
  useEffect,
  useState,
} from 'react';
import styled from 'styled-components';

import { Dots } from 'teleport/Assist/Dots';

import { RunIcon } from '../../../Icons/RunIcon';
import { ExecOutput, MessageContent } from '../../../services/messages';

import { ExecResult } from './ExecResult';

interface ActionsProps {
  contents: MessageContent[];
  scrollTextarea: () => void;
}

const Container = styled.div`
  width: 100%;
  margin-top: 30px;
`;

const Title = styled.div`
  font-size: 15px;
  margin-bottom: 10px;
`;

const Buttons = styled.div`
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
`;

const Button = styled.div`
  display: flex;
  padding: 10px 20px 10px 15px;
  border-radius: 5px;
  font-weight: bold;
  font-size: 18px;
  align-items: center;
  margin-left: 20px;
  cursor: pointer;

  svg {
    margin-right: 5px;
  }
`;

const ButtonRun = styled(Button)<{ disabled: boolean }>`
  border: 2px solid ${p => (p.disabled ? '#cccccc' : '#20b141')};
  opacity: ${p => (p.disabled ? 0.8 : 1)};
  cursor: ${p => (p.disabled ? 'not-allowed' : 'pointer')};

  &:hover {
    background: ${p => (p.disabled ? 'none' : '#20b141')};
  }
`;

const ButtonCancel = styled(Button)`
  color: #e85654;
`;

const Spacer = styled.div`
  text-align: center;
  padding: 10px 0;
  font-size: 14px;
`;

const LoadingContainer = styled.div`
  display: flex;
  justify-content: center;
  margin: 30px 0;
`;

export function Actions(props: PropsWithChildren<ActionsProps>) {
  const children: ReactNode[] = [];
  const [loading, setLoading] = useState(false);
  const [result] = useState<ExecOutput | null>(null);

  Children.forEach(props.children, (child, index) => {
    children.push(child);
    children.push(<Spacer key={`spacer-${index}`}>and</Spacer>);
  });

  useEffect(() => {
    props.scrollTextarea();
  }, [loading, props.scrollTextarea]);

  const run = useCallback(async () => {
    if (loading) {
      return;
    }

    setLoading(true);
  }, [props.contents, loading]);

  return (
    <Container>
      {!result && <Title>Teleport would like to</Title>}

      {children.slice(0, -1)}

      {!result && (
        <Buttons>
          {!loading && <ButtonCancel>Cancel</ButtonCancel>}
          <ButtonRun onClick={() => run()} disabled={loading}>
            <RunIcon size={30} />
            {loading ? 'Running' : 'Run'}
          </ButtonRun>
        </Buttons>
      )}

      {loading && (
        <LoadingContainer>
          <Dots />
        </LoadingContainer>
      )}

      {result && <ExecResult result={result} />}
    </Container>
  );
}
