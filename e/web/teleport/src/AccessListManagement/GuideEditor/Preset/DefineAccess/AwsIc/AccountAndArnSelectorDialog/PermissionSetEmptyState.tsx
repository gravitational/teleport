import { Box, Mark, Text } from 'design';

import { AccountOption } from './types';

export function PermissionSetEmptyState({
  selectedAccounts,
  hasWildCard,
}: {
  selectedAccounts: AccountOption[];
  hasWildCard: boolean;
}) {
  let reason = (
    <>
      Select an account on the left to see available permission sets.
      <br />
      <br />
      If selecting more than one account, only permission sets{' '}
      <Mark>shared</Mark> between selected accounts will be listed.
    </>
  );

  if (hasWildCard) {
    reason = (
      <>
        There are no <Mark>shared</Mark> permission sets available for any
        account.
      </>
    );
  } else if (selectedAccounts.length === 1) {
    reason = (
      <>There are no permission sets available for the selected account.</>
    );
  } else if (selectedAccounts.length > 1) {
    reason = (
      <>
        There are no <Mark>shared</Mark> permission sets between the selected
        accounts. Try a different selection.
      </>
    );
  }

  return (
    <Box
      height="100%"
      css={`
        background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
        border-left: 1px solid
          ${p => p.theme.colors.interactive.tonal.neutral[0]};
        overflow: auto;
      `}
      p={3}
    >
      <Text bold mb={3}>
        2. Select Permission Sets
      </Text>
      <Text pl={3}>{reason}</Text>
    </Box>
  );
}
