import React from 'react';

import {
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttempt,
} from 'shared/hooks/useAsync';

import { RequestDelete } from './RequestDelete';

export default {
  title: 'TeleportE/AccessRequests/RequestDelete',
};

export const Loaded = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      deleteRequestAttempt={makeEmptyAttempt()}
    />
  );
};

export const Processing = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      deleteRequestAttempt={makeProcessingAttempt()}
    />
  );
};

export const Failed = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      deleteRequestAttempt={makeErrorAttempt(new Error('server error'))}
    />
  );
};

export const Approved = () => {
  return (
    <RequestDelete
      {...props}
      deleteRequestAttempt={makeEmptyAttempt()}
      requestState="APPROVED"
    />
  );
};

const props = {
  requestId: '5ee98d44-de9d-5103-a7cd-072b1ff76253',
  user: 'admin',
  roles: ['dba'],
  onDelete: () => null,
  onClose: () => null,
};
