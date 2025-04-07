/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { StoryObj } from '@storybook/react';
import { delay, http, HttpResponse } from 'msw';
import { useEffect, useState } from 'react';

import { Info } from 'design/Alert';
import { ButtonPrimary } from 'design/Button';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import Text from 'design/Text';
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { wait } from 'shared/utils/wait';

import useResources from 'teleport/components/useResources';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { RoleVersion } from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';
import { Access } from 'teleport/services/user';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { RoleEditor } from './RoleEditor';
import { RoleEditorDialog } from './RoleEditorDialog';
import { unableToUpdatePreviewMessage } from './Shared';
import { withDefaults } from './StandardEditor/withDefaults';

const defaultIsPolicyEnabled = cfg.isPolicyEnabled;
const defaultGetAccessGraphRoleTesterEnabled =
  storageService.getAccessGraphRoleTesterEnabled;

export default {
  title: 'Teleport/Roles/Role Editor',
  decorators: [
    (Story, { parameters }) => {
      const ctx = createTeleportContext();
      if (parameters.acl) {
        ctx.storeUser.getRoleAccess = () => parameters.acl;
      }
      if (parameters.roleTesterEnabled) {
        cfg.isPolicyEnabled = true;
        storageService.getAccessGraphRoleTesterEnabled = () => true;
      }
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isPolicyEnabled = defaultIsPolicyEnabled;
          storageService.getAccessGraphRoleTesterEnabled =
            defaultGetAccessGraphRoleTesterEnabled;
        };
      }, []);
      return (
        <TeleportContextProvider ctx={ctx}>
          <Flex flexDirection="column" width="550px" height="800px">
            <Story />
          </Flex>
        </TeleportContextProvider>
      );
    },
  ],
};

const yamlifyHandler = http.post(
  cfg.getYamlStringifyUrl(YamlSupportedResourceKind.Role),
  () => HttpResponse.json({ yaml: dummyRoleYaml })
);

const parseHandler = http.post(
  cfg.getYamlParseUrl(YamlSupportedResourceKind.Role),
  () =>
    HttpResponse.json({
      resource: withDefaults({
        metadata: { name: 'dummy-role' },
        version: RoleVersion.V7,
      }),
    })
);

const serverErrorResponse = async () =>
  HttpResponse.json({ error: { message: 'server error' } }, { status: 500 });

export const NewRole: StoryObj = {
  render() {
    return <RoleEditor />;
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
  },
};

export const ExistingRole: StoryObj = {
  render() {
    return (
      <RoleEditor
        originalRole={{
          object: withDefaults({ metadata: { name: 'dummy-role' } }),
          yaml: dummyRoleYaml,
        }}
      />
    );
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
  },
};

export const yamlifyProcessing: StoryObj = {
  render() {
    return (
      <>
        <Info>Switch to the YAML tab to see the processing state</Info>
        <RoleEditor />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [
        http.post(
          cfg.getYamlStringifyUrl(YamlSupportedResourceKind.Role),
          async () => await delay('infinite')
        ),
        parseHandler,
      ],
    },
  },
};

export const yamlifyError: StoryObj = {
  render() {
    return (
      <>
        <Info>Switch to the YAML tab to see the error state</Info>
        <RoleEditor />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [
        http.post(
          cfg.getYamlStringifyUrl(YamlSupportedResourceKind.Role),
          serverErrorResponse
        ),
        parseHandler,
      ],
    },
  },
};

export const parseProcessing: StoryObj = {
  render() {
    return (
      <>
        <Info>Switch to the Standard tab to see the processing state</Info>
        <RoleEditor
          originalRole={{
            object: withDefaults({
              metadata: { name: 'dummy-role' },
              spec: { deny: { node_labels: { foo: ['bar'] } } },
            }),
            yaml: dummyUnsupportedRoleYaml,
          }}
        />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [
        yamlifyHandler,
        http.post(
          cfg.getYamlParseUrl(YamlSupportedResourceKind.Role),
          async () => await delay('infinite')
        ),
      ],
    },
  },
};

export const parseError: StoryObj = {
  render() {
    return (
      <>
        <Info>Switch to the Standard tab to see the error state</Info>
        <RoleEditor
          originalRole={{
            object: withDefaults({
              metadata: { name: 'dummy-role' },
              spec: { deny: { node_labels: { foo: ['bar'] } } },
            }),
            yaml: dummyUnsupportedRoleYaml,
          }}
        />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [
        yamlifyHandler,
        http.post(
          cfg.getYamlParseUrl(YamlSupportedResourceKind.Role),
          serverErrorResponse
        ),
      ],
    },
  },
};

export const saving: StoryObj = {
  render() {
    return (
      <>
        <Info>Edit and save the role to see the saving state</Info>
        <RoleEditor
          originalRole={{
            object: withDefaults({ metadata: { name: 'dummy-role' } }),
            yaml: dummyRoleYaml,
          }}
          onSave={() => delay('infinite')}
        />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
  },
};

export const savingError: StoryObj = {
  render() {
    return (
      <>
        <Info>Edit and save the role to see the error state</Info>
        <RoleEditor
          originalRole={{
            object: withDefaults({ metadata: { name: 'dummy-role' } }),
            yaml: dummyRoleYaml,
          }}
          onSave={async () => {
            throw new Error('Server error', {
              cause: new Error('Unexpected rack explosion'),
            });
          }}
        />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
  },
};

export const noAccess: StoryObj = {
  render() {
    return (
      <RoleEditor
        originalRole={{
          object: withDefaults({ metadata: { name: 'dummy-role' } }),
          yaml: dummyRoleYaml,
        }}
      />
    );
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
    acl: {
      list: true,
      create: false,
      edit: false,
      read: true,
      remove: false,
    } as Access,
  },
};

export const Dialog: StoryObj = {
  render() {
    const [open, setOpen] = useState(false);
    const resources = useResources([], {});
    return (
      <>
        <ButtonPrimary onClick={() => setOpen(true)}>Open</ButtonPrimary>
        <RoleEditorDialog
          resources={resources}
          open={open}
          onClose={() => setOpen(false)}
          onSave={async () => setOpen(false)}
        />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
  },
};

export const DialogWithPolicyEnabled: StoryObj = {
  render() {
    const [open, setOpen] = useState(false);
    const resources = useResources([], {});
    const [roleDiffAttempt, mockGetDiff] = useAsync(() => wait(1000));
    return (
      <>
        <ButtonPrimary onClick={() => setOpen(true)}>Open</ButtonPrimary>
        <RoleEditorDialog
          resources={resources}
          roleDiffProps={getRoleDiffProps(roleDiffAttempt, mockGetDiff)}
          open={open}
          onClose={() => setOpen(false)}
          onSave={async () => setOpen(false)}
        />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
    roleTesterEnabled: true,
  },
};

export const AccessGraphError: StoryObj = {
  render() {
    const [open, setOpen] = useState(false);
    const resources = useResources([], {});
    const [roleDiffAttempt, mockGetDiff] = useAsync(async () => {
      await wait(1000);
      throw new Error(unableToUpdatePreviewMessage, {
        cause: new Error("There's a raccoon in the router"),
      });
    });
    return (
      <>
        <ButtonPrimary onClick={() => setOpen(true)}>Open</ButtonPrimary>
        <RoleEditorDialog
          resources={resources}
          roleDiffProps={getRoleDiffProps(roleDiffAttempt, mockGetDiff)}
          open={open}
          onClose={() => setOpen(false)}
          onSave={async () => setOpen(false)}
        />
      </>
    );
  },
  parameters: {
    msw: {
      handlers: [yamlifyHandler, parseHandler],
    },
    roleTesterEnabled: true,
  },
};

const getRoleDiffProps = (
  roleDiffAttempt: Attempt<unknown>,
  getRoleDiff: () => void
) => ({
  roleDiffElement: (
    <Flex
      flex="1"
      alignItems="center"
      justifyContent="center"
      flexDirection="column"
      gap="2"
    >
      <Text typography="h1">Access Graph Placeholder</Text>
      {roleDiffAttempt.status === 'processing' && <Indicator />}
    </Flex>
  ),
  updateRoleDiff: getRoleDiff,
  roleDiffAttempt,
});

const dummyRoleYaml = `kind: role
metadata:
  name: dummy-role
spec:
  allow: {}
  deny: {}
  options:
    cert_format: standard
    create_db_user: false
    create_desktop_user: false
    desktop_clipboard: true
    desktop_directory_sharing: true
    enhanced_recording:
    - command
    - network
    forward_agent: false
    idp:
      saml:
        enabled: true
    max_session_ttl: 30h0m0s
    pin_source_ip: false
    ssh_port_forwarding:
      remote:
        enabled: false
      local:
        enabled: false
    record_session:
      default: best_effort
      desktop: true
    ssh_file_copy: true
version: v7
`;

// This role contains an unsupported field. Not that it really matters, since
// in the story, we mock out the YAML-JSON translation process.
const dummyUnsupportedRoleYaml = `kind: role
metadata:
  name: dummy-role
  unsupportedField: unsupported
spec:
  allow: {}
  deny: {}
  options:
    cert_format: standard
    create_db_user: false
    create_desktop_user: false
    desktop_clipboard: true
    desktop_directory_sharing: true
    enhanced_recording:
    - command
    - network
    forward_agent: false
    idp:
      saml:
        enabled: true
    max_session_ttl: 30h0m0s
    pin_source_ip: false
    ssh_port_forwarding:
      remote:
        enabled: false
      local:
        enabled: false
    record_session:
      default: best_effort
      desktop: true
    ssh_file_copy: true
version: v7
`;
