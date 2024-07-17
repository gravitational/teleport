import React from 'react';
import { Card, H1, Text } from 'design';

export default function InvalidLink() {
  return (
    <Card
      width="540px"
      color="text.main"
      p={6}
      bg="levels.elevated"
      mt={6}
      mx="auto"
    >
      <H1 textAlign="center" mb={3}>
        Invalid Recovery Link
      </H1>
      <Text typography="paragraph" mb="2" textAlign="center">
        This recovery link is invalid or has expired.
      </Text>
      <Text typography="paragraph" textAlign="center">
        If you believe this is a mistake, please contact us at:
        support@goteleport.com
      </Text>
    </Card>
  );
}
