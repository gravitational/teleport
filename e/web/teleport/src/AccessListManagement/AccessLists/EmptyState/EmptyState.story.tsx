import React from 'react';
import { MemoryRouter } from 'react-router';

import { EmptyState } from './EmptyState';

export default {
  title: 'Teleport/AccessLists',
};

export const Story = () => {
  return (
    <MemoryRouter>
      <EmptyState />
    </MemoryRouter>
  );
};
Story.storyName = 'EmptyState';
