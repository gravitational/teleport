import React from 'react';
import { RequestDelete } from './RequestDelete';

export default {
  title: 'TeleportE/Workflow/RequestDelete',
};

export const Loaded = () => {
  return (
    <RequestDelete {...props} requestState="PENDING" attempt={{ status: '' }} />
  );
};

export const Processing = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      attempt={{ status: 'processing' }}
    />
  );
};

export const Failed = () => {
  return (
    <RequestDelete
      {...props}
      requestState="PENDING"
      attempt={{ status: 'failed', statusText: 'server error' }}
    />
  );
};

export const Approved = () => {
  return (
    <RequestDelete
      {...props}
      attempt={{ status: '' }}
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
