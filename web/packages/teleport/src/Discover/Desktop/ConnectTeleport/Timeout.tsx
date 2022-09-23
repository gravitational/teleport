import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

interface TimeoutProps {
  timeout: number; // ms
}

const Container = styled.div`
  margin-top: 20px;
`;

export function Timeout(props: TimeoutProps) {
  const [, setCount] = useState(0);

  useEffect(() => {
    const interval = window.setInterval(() => {
      if (Date.now() >= props.timeout) {
        clearInterval(interval);
      }

      setCount(count => count + 1);
    }, 1000);

    return () => clearInterval(interval);
  }, [props.timeout]);

  const { minutes, seconds } = millisecondsToMinutesSeconds(
    props.timeout - Date.now()
  );

  return (
    <Container>
      This script is valid for another {minutes}:{seconds}.
    </Container>
  );
}

function millisecondsToMinutesSeconds(ms: number) {
  if (ms < 0) {
    return { minutes: 0, seconds: 0 };
  }

  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000)
    .toFixed(0)
    .padStart(2, '0');

  return { minutes, seconds };
}
