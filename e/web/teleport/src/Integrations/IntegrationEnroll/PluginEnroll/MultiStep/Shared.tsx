import React from 'react';
import { H1 } from 'design';

export const Header = ({ header }: { header: string }) => (
  <H1 mb={2}>{header}</H1>
);
