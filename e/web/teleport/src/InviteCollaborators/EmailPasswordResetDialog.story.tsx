import React from 'react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import EmailPasswordResetDialog from './EmailPasswordResetDialog';

export default {
  title: 'TeleportE/InviteCollaborators/EmailPasswordReset',
};

export const DialogEmailLike = () => {
  const ctx = createTeleportContext() as any;
  ctx.cloudService = { sendTeleportCredentialReset: () => Promise.resolve([]) };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <EmailPasswordResetDialog
          onClose={() => {}}
          username={'alice@example.com'}
        />
      </ContextProvider>
    </MemoryRouter>
  );
};

export const DialogNotEmailLike = () => {
  const ctx = createTeleportContext() as any;
  ctx.cloudService = { sendTeleportCredentialReset: () => Promise.resolve([]) };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <EmailPasswordResetDialog onClose={() => {}} username={'alice'} />
      </ContextProvider>
    </MemoryRouter>
  );
};
