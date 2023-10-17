import React from 'react';
import styled from 'styled-components';

import Select, { Option } from 'shared/components/Select';

export type Days = 7 | 30 | 90;

interface TimeframeProps {
  days: Days;
  onChange: (days: Days) => void;
}

const StyledSelect = styled.div`
  flex: 0 0 150px;
`;

export function Timeframe(props: TimeframeProps) {
  const timeframes: Option<Days>[] = [
    {
      label: 'Last 7 days',
      value: 7,
    },
    {
      label: 'Last 30 days',
      value: 30,
    },
    {
      label: 'Last 90 days',
      value: 90,
    },
  ];

  const selectedTimeframe = timeframes.find(
    timeframe => timeframe.value === props.days
  );

  function handleChange(timeframe: Option<Days>) {
    props.onChange(timeframe.value);
  }

  return (
    <StyledSelect>
      <Select
        onChange={handleChange}
        value={selectedTimeframe}
        options={timeframes}
      />
    </StyledSelect>
  );
}
