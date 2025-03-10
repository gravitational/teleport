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

import useResources from 'teleport/components/useResources';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { RoleVersion } from 'teleport/services/resources';
import { Access } from 'teleport/services/user';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { RoleEditor } from './RoleEditor';
import { RoleEditorDialog } from './RoleEditorDialog';
import { withDefaults } from './StandardEditor/withDefaults';

const defaultIsPolicyEnabled = cfg.isPolicyEnabled;

export default {
  title: 'Teleport/Roles/Role Editor',
  decorators: [
    (Story, { parameters }) => {
      const ctx = createTeleportContext();
      if (parameters.acl) {
        ctx.storeUser.getRoleAccess = () => parameters.acl;
      }
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isPolicyEnabled = defaultIsPolicyEnabled;
        };
      }, []);
      return (
        <TeleportContextProvider ctx={ctx}>
          <Flex flexDirection="column" width="700px" height="800px">
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
        <Info>Save the role to see the saving state</Info>
        <RoleEditor onSave={() => delay('infinite')} />
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
        <Info>Save the role to see the error state</Info>
        <RoleEditor
          onSave={async () => {
            throw new Error('server error');
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
    cfg.isPolicyEnabled = true;
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
