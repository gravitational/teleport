import React from 'react';

import { NoAccessState } from './NoAccessState';

export default {
  title: 'Teleport/AccessLists/NoAccessState',
};

export const Create = () => <NoAccessState action="create" />;
export const List = () => <NoAccessState action="list" />;
export const Read = () => <NoAccessState action="read" />;
