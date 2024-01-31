import React from 'react';
import { Text } from 'design';

export const Header = ({ header }: { header: string }) => (
  <Text fontSize={7} mb={2}>
    {header}
  </Text>
);
