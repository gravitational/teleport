/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { delay, http, HttpResponse } from 'msw';

import { Flex } from 'design';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { ContextProvider } from 'teleport';
import { InfoGuideSidePanel } from 'teleport/components/SlidingSidePanel/InfoGuideSidePanel';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { Acl } from 'teleport/services/user/types';

import {
  mockManagedUpdatesAgentsDropped,
  mockManagedUpdatesHaltOnError,
  mockManagedUpdatesImmediate,
  mockManagedUpdatesNotConfigured,
  mockManagedUpdatesTimeBased,
  mockManagedUpdatesWithOrphaned,
} from './fixtures';
import { ManagedUpdates } from './ManagedUpdates';

export default {
  title: 'Teleport/ManagedUpdates',
};

const StoryContainer = ({
  children,
  customAcl,
}: {
  children: React.ReactNode;
  customAcl?: Acl;
}) => {
  const ctx = createTeleportContext({ customAcl });
  return (
    <ContextProvider ctx={ctx}>
      <InfoGuidePanelProvider>
        <Flex
          height="100vh"
          flexDirection="column"
          css={`
            // Override the top offset for slideout panel since there's no navbar in storybook.
            & + div {
              top: 0;
            }
          `}
        >
          {children}
        </Flex>
        <InfoGuideSidePanel />
      </InfoGuidePanelProvider>
    </ContextProvider>
  );
};

export function LoadedTimeBased() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
LoadedTimeBased.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesTimeBased);
      }),
    ],
  },
};

export function LoadedHaltOnError() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
LoadedHaltOnError.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesHaltOnError);
      }),
    ],
  },
};

export function LoadedAgentsDropped() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
LoadedAgentsDropped.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesAgentsDropped);
      }),
    ],
  },
};

export function WithOrphanedAgents() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
WithOrphanedAgents.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesWithOrphaned);
      }),
    ],
  },
};

export function ImmediateSchedule() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
ImmediateSchedule.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesImmediate);
      }),
    ],
  },
};

export function NotConfigured() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
NotConfigured.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesNotConfigured);
      }),
    ],
  },
};

export function Loading() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
Loading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), async () => {
        await delay('infinite');
        return HttpResponse.json({});
      }),
    ],
  },
};

export function Error() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
Error.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json({ message: 'some error' }, { status: 500 });
      }),
    ],
  },
};

export function ActionLoading() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
ActionLoading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesHaltOnError);
      }),
      http.post(
        '/v1/webapi/managedupdates/groups/:groupName/:action',
        async () => {
          await delay('infinite');
          return HttpResponse.json({});
        }
      ),
    ],
  },
};

export function ActionError() {
  return (
    <StoryContainer>
      <ManagedUpdates />
    </StoryContainer>
  );
}
ActionError.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesHaltOnError);
      }),
      http.post('/v1/webapi/managedupdates/groups/:groupName/:action', () => {
        return HttpResponse.json(
          { message: 'error performing group action' },
          { status: 403 }
        );
      }),
    ],
  },
};

const noPermissionsAcl = makeAcl({
  autoUpdateConfig: defaultAccess,
  autoUpdateVersion: defaultAccess,
  autoUpdateAgentRollout: defaultAccess,
});

export function NoPermissions() {
  return (
    <StoryContainer customAcl={noPermissionsAcl}>
      <ManagedUpdates />
    </StoryContainer>
  );
}
NoPermissions.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesTimeBased);
      }),
    ],
  },
};

const toolsOnlyAcl = makeAcl({
  autoUpdateConfig: { ...defaultAccess, read: true },
  autoUpdateVersion: { ...defaultAccess, read: true },
  autoUpdateAgentRollout: defaultAccess,
});

export function ToolsPermissionsOnly() {
  return (
    <StoryContainer customAcl={toolsOnlyAcl}>
      <ManagedUpdates />
    </StoryContainer>
  );
}
ToolsPermissionsOnly.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesTimeBased);
      }),
    ],
  },
};

const rolloutOnlyAcl = makeAcl({
  autoUpdateConfig: defaultAccess,
  autoUpdateVersion: defaultAccess,
  autoUpdateAgentRollout: { ...defaultAccess, read: true },
});

export function RolloutPermissionsOnly() {
  return (
    <StoryContainer customAcl={rolloutOnlyAcl}>
      <ManagedUpdates />
    </StoryContainer>
  );
}
RolloutPermissionsOnly.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getManagedUpdatesUrl(), () => {
        return HttpResponse.json(mockManagedUpdatesTimeBased);
      }),
    ],
  },
};
