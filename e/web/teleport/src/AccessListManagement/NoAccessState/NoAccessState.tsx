import React from 'react';
import { Text, Box, Flex } from 'design';
import TextEditor from 'shared/components/TextEditor';

import { crud } from '.';

export function NoAccessState({
  action,
}: {
  action: 'create' | 'list' | 'read';
}) {
  return (
    <Box>
      <Text>
        You don’t have sufficient permissions to {action} Access Lists. Reach
        out to your Teleport administrator to request additional permissions:
      </Text>
      <Flex minHeight="245px" mt={3}>
        <TextEditor
          readOnly={true}
          data={[{ content: crud, type: 'yaml' }]}
          bg="levels.deep"
        />
      </Flex>
    </Box>
  );
}
