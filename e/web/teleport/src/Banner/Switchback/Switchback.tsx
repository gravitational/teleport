import React from 'react';
import styled from 'styled-components';
import { Flex, Text, Box } from 'design';
import { pluralize } from 'teleport/lib/util';
import useTeleport from 'e-teleport/useTeleportE';
import useSwitchback, { State } from './useSwitchback';
import ErrorAlert from './ErrorAlert';

export default function Container() {
  const ctx = useTeleport();
  const state = useSwitchback(ctx);

  return <Switchback {...state} />;
}

export function Switchback({
  assumedRoles,
  time,
  btnSetting,
  attempt,
  onErrorConfirm,
}: State) {
  const roleText = pluralize(assumedRoles.length, 'role');
  const durationTxt = getDurationText(time.hours, time.minutes, time.seconds);
  const roles = assumedRoles.join(', ');

  return (
    <Flex height="48px" bg="secondary.light" justifyContent="center">
      {attempt.status === 'failed' && (
        <ErrorAlert err={attempt.statusText} onClose={onErrorConfirm} />
      )}
      <Flex alignItems="center">
        <Box
          borderRadius="20px"
          py={1}
          px={3}
          mr={2}
          color="secondary.main"
          bg="light"
          style={{
            fontWeight: '500',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            maxWidth: '200px',
            whiteSpace: 'nowrap',
          }}
          title={roles}
        >
          {roles}
        </Box>
        <Text mr={1}>
          {roleText} assumed, expires in {durationTxt}
        </Text>
        <StyledButtonLink
          onClick={btnSetting.func}
          disabled={attempt.status === 'processing'}
        >
          {btnSetting.text}
        </StyledButtonLink>
      </Flex>
    </Flex>
  );
}

function getDurationText(hrs: number, mins: number, secs: number) {
  if (!hrs && !mins) {
    return `${secs} secs`;
  }

  const hrText = pluralize(hrs, 'hr');
  const minText = pluralize(mins, 'min');

  if (!hrs) {
    return `${mins} ${minText}`;
  }

  if (hrs && !mins) {
    return `${hrs} ${hrText}`;
  }

  return `${hrs} ${hrText} and ${mins} ${minText}`;
}

const StyledButtonLink = styled.button`
  color: ${props => props.theme.colors.text.primary};
  background: none;
  text-decoration: underline;
  text-transform: none;
  padding: 8px;
  outline: none;
  border: none;
  border-radius: 4px;

  &:hover,
  &:focus {
    background: #793cff;
    cursor: pointer;
  }

  &:disabled {
    background: #793cff;
    color: ${props => props.theme.colors.action.disabled};
  }
`;
