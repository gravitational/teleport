import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { ToastNotificationProvider } from 'shared/components/ToastNotification';

import cfg from 'e-teleport/config';
import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { InviteCollaboratorsDialog } from './InviteCollaboratorsDialog';

export default {
  title: 'TeleportE/InviteCollaborators',
};

export const Dialog: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json([
            { name: 'apple' },
            { name: 'banana' },
            {
              name: 'carrot',
              roles: ['reviewer', 'auditor'],
              allTraits: { fruit: ['carrot'] },
            },
          ]);
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json({
            items: [
              { name: 'admin', description: 'admin' },
              { name: 'auditor', description: 'auditor' },
              { name: 'reviewer', description: 'reviewer' },
              { name: 'access', description: 'access' },
              { name: 'editor' },
            ],
          });
        }),
      ],
    },
  },
  render() {
    const ctx = createTeleportContext() as any;
    ctx.cloudService = { sendTeleportInvite: () => Promise.resolve([]) };

    return (
      <MemoryRouter>
        <ToastNotificationProvider>
          <ContextProvider ctx={ctx}>
            <InviteCollaboratorsDialog onClose={() => {}} open={true} />
          </ContextProvider>
        </ToastNotificationProvider>
      </MemoryRouter>
    );
  },
};

export const DialogError: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.oss.getUsersUrl(), () => {
          return HttpResponse.json(
            {
              message: 'testing error for getUsers()',
            },
            { status: 500 }
          );
        }),
        http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
          return HttpResponse.json(
            {
              message: 'testing error for getRoles()',
            },
            { status: 500 }
          );
        }),
      ],
    },
  },
  render() {
    const ctx = createTeleportContext() as any;
    ctx.cloudService = { sendTeleportInvite: () => Promise.resolve([]) };

    return (
      <MemoryRouter>
        <ToastNotificationProvider>
          <ContextProvider ctx={ctx}>
            <InviteCollaboratorsDialog onClose={() => {}} open={true} />
          </ContextProvider>
        </ToastNotificationProvider>
      </MemoryRouter>
    );
  },
};

export const DialogSpinner = () => {
  const ctx = createTeleportContext() as any;
  ctx.cloudService = { sendTeleportInvite: () => Promise.resolve([]) };
  ctx.userService = {
    fetchUsers: () => new Promise(() => {}),
  };
  ctx.resourceService = {
    fetchRoles: () => new Promise(() => {}),
  };

  return (
    <MemoryRouter>
      <ToastNotificationProvider>
        <ContextProvider ctx={ctx}>
          <InviteCollaboratorsDialog onClose={() => {}} open={true} />
        </ContextProvider>
      </ToastNotificationProvider>
    </MemoryRouter>
  );
};
