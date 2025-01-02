import { components } from 'react-select';
import styled from 'styled-components';

import Box from 'design/Box';
import Select, { type Option } from 'shared/components/Select';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import { CtaEvent } from 'teleport/services/userEvent';

export type Days = 0 | 7 | 30 | 90 | 120;

interface TimeframeProps {
  days: Days;
  onChange: (days: Days) => void;
}

const StyledSelect = styled.div`
  flex: 0 0 250px;
  .react-select__option--is-disabled {
    opacity: 0.4;
    pointer-events: none;
    &:hover {
      background-color: none;
    }
  }

  .react-select__option__btn {
    background: none;
    &:hover {
      cursor: auto;
      background: none;
    }
  }
`;

type DurationOption = Option<Days> & {
  isDisabled?: boolean;
};

// Note: today the limit for access monitoring is set on entitlements as 30, but we don't leverage the limit here
export function Timeframe(props: TimeframeProps) {
  const limited = cfg.entitlements.AccessMonitoring.limit > 0;
  const timeframes: DurationOption[] = [
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
      isDisabled: limited,
    },
    {
      label: 'Last 120 days',
      value: 120,
      isDisabled: limited,
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
        components={{
          Option: OptionComponent,
          MenuList: MenuListComponent,
        }}
      />
    </StyledSelect>
  );
}

const OptionComponent = props => {
  return (
    <components.Option
      {...props}
      className={`react-select__option${!props.value ? '__btn' : ''}`}
    >
      {props.label}
    </components.Option>
  );
};

const MenuListComponent = props => {
  return (
    <>
      <components.MenuList {...props}>{props.children}</components.MenuList>
      {cfg.entitlements.AccessMonitoring.limit > 0 && (
        <Box
          p={2}
          css={`
            background-color: transparent;
          `}
        >
          <ButtonLockedFeature
            event={CtaEvent.CTA_ACCESS_MONITORING}
            noIcon={true}
          >
            Unlock higher range with Teleport Identity
          </ButtonLockedFeature>
        </Box>
      )}
    </>
  );
};
