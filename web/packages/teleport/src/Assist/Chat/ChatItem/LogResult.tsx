import React, { useState } from 'react';
import styled from 'styled-components';

import { LogResultContent } from 'teleport/Assist/services/messages';

interface LogResultProps {
  content: LogResultContent;
}

const Container = styled.div`
  font-size: 15px;
  padding-bottom: 16px;
  width: inherit;
`;

const Tabs = styled.div`
  display: flex;
`;

const Tab = styled.div<{ active: boolean }>`
  background: ${p => (p.active ? '#9966ff' : 'none')};
  color: ${p => (p.active ? 'white' : 'black')};
  font-weight: ${p => (p.active ? 700 : 400)};
  border-radius: 5px;
  padding: 7px 15px;
  cursor: pointer;
`;

const Logs = styled.div`
  background: #212121;
  color: white;
  border-radius: 7px;
  margin-top: 20px;
  padding: 5px 0;
  width: inherit;

  pre {
    counter-reset: line;
    margin: 0;
    font-size: 14px;
    counter-set: line ${p => p.startNumber || 0};
    overflow-x: auto;
  }
`;

const LineSpan = styled.span`
  display: block;
  background: ${p => (p.active ? '#181818' : 'none')};
  opacity: ${p => (p.active ? 1 : 0.4)};

  &:before {
    counter-increment: line;
    content: counter(line);
    display: inline-block;
    padding: 0 0.5em;
    margin-right: 0.5em;
    width: 40px;
    color: ${p => (p.active ? 'white' : '#888888')};
    text-align: right;
  }
`;

export function LogResult(props: LogResultProps) {
  const [activeTabIndex, setActiveTabIndex] = useState(0);

  const tabs = props.content.logs.map((log, index) => (
    <Tab
      key={index}
      active={index === activeTabIndex}
      onClick={() => setActiveTabIndex(index)}
    >
      {log.nodeName}
    </Tab>
  ));

  return (
    <Container>
      <Tabs>{tabs}</Tabs>

      <Logs startNumber={props.content.logs[activeTabIndex].lineNumberStart}>
        <pre data-scrollbar="default">
          {props.content.logs[activeTabIndex].contents
            .split('\n')
            .map((line, index) => (
              <LineSpan
                key={index}
                active={props.content.logs[
                  activeTabIndex
                ].highlightedLines.includes(index + 1)}
              >
                {line}
              </LineSpan>
            ))}
        </pre>
      </Logs>
    </Container>
  );
}
