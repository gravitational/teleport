import { useTheme } from 'styled-components';

import { Banner, Box, Flex } from 'design';
import { getDurationText } from 'shared/utils/getDurationText';
import { pluralize } from 'shared/utils/text';

import useTeleport from 'e-teleport/useTeleportE';

import ErrorAlert from './ErrorAlert';
import useSwitchback, { State } from './useSwitchback';

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
  const theme = useTheme();

  const roleText = pluralize(assumedRoles.length, 'role');
  const durationTxt = getDurationText(time.hours, time.minutes, time.seconds);
  const roles = assumedRoles.join(', ');

  return (
    <Banner
      kind="primary"
      primaryAction={{ content: btnSetting.text, onClick: btnSetting.func }}
    >
      {attempt.status === 'failed' && (
        <ErrorAlert err={attempt.statusText} onClose={onErrorConfirm} />
      )}
      <Flex alignItems="center">
        <Box
          borderRadius="20px"
          py={0}
          px={2}
          mr={2}
          color={theme.colors.text.primaryInverse}
          bg={theme.colors.interactive.solid.primary.default}
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
        {roleText} assumed, expires in {durationTxt}
      </Flex>
    </Banner>
  );
}
