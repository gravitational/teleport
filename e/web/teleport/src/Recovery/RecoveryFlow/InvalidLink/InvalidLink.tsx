import React from 'react';
import { Card, Text } from 'design';

export default function InvalidLink() {
  return (
    <Card width="540px" color="text.onLight" p={6} bg="light" mt={6} mx="auto">
      <Text typography="h1" textAlign="center" fontSize={8} color="text" mb={3}>
        Invalid Recovery Link
      </Text>
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
