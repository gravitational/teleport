import React from 'react';
import { Text, Box } from 'design';

export function NoAccessState({ action }: { action?: 'create' }) {
  return (
    <Box px={4} py={4} bg="levels.surface" borderRadius={3}>
      {action === 'create' ? (
        <Text>Only Teleport Administrator's can create new Access Lists.</Text>
      ) : (
        <Text>
          Access listing is only available for owners or members of Access Lists
          or Teleport Administrator's.
        </Text>
      )}
    </Box>
  );
}
