import { Text, Box } from 'design';

export function NoAccessState({ action }: { action?: 'create' }) {
  return (
    <Box px={4} py={4} bg="levels.surface" borderRadius={3}>
      {action === 'create' ? (
        <Text>Only Teleport administrators can create new Access Lists.</Text>
      ) : (
        <Text>
          Access lists can only be viewed by their owners, members, and other
          Teleport administrators.
        </Text>
      )}
    </Box>
  );
}
