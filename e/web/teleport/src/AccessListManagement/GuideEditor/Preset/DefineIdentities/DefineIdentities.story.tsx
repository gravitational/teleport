import { Meta } from '@storybook/react-vite';
import { useEffect, useRef } from 'react';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { AccessGraphDemoProvider } from 'e-teleport/Roles/AccessGraphDemoContext';

import { StandardRoleConditions } from '../role/conditions';
import { LabelBasedResourceAccessFields } from '../role/listaccess';
import { AppIdentities } from '../role/resources/app';
import { fetchUnifiedResources, makeHandlers } from '../TestHelper/mocks';
import { Provider } from '../TestHelper/Provider';
import { DefineIdentities as MainComponent } from './DefineIdentities';

type StoryProps = {
  identitiesRequired: boolean;
  app: boolean;
  db: boolean;
  kubernetes: boolean;
  server: boolean;
  windowsDesktop: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'TeleportE/AccessLists/Guide',
  argTypes: {
    identitiesRequired: {
      control: 'boolean',
      description:
        'If false, shows empty state and other flags have no effect.',
    },
    app: { control: 'boolean' },
    db: { control: 'boolean' },
    kubernetes: { control: 'boolean' },
    server: { control: 'boolean' },
    windowsDesktop: { control: 'boolean' },
  },
  args: {
    identitiesRequired: true,
    app: true,
    db: false,
    kubernetes: false,
    server: false,
    windowsDesktop: false,
  },
  parameters: {
    msw: {
      handlers: makeHandlers([fetchUnifiedResources('get', [])]),
    },
  },
};
export default meta;

type WithAccessDefinitionProps = React.PropsWithChildren<{
  labelFields: LabelBasedResourceAccessFields[];
  appIdentityFields?: (keyof AppIdentities)[];
  allPagesFetched?: boolean;
}>;

const WithAccessDefinition: React.FC<WithAccessDefinitionProps> = ({
  children,
  labelFields,
  appIdentityFields,
  allPagesFetched = true,
}) => {
  const { guideEditor } = useAccessListManagementContext();
  const { standardRoleState } = guideEditor;
  const initialized = useRef(false);

  useEffect(() => {
    if (initialized.current) {
      return;
    }
    initialized.current = true;

    const updatedConditions: StandardRoleConditions = {
      ...standardRoleState.roleConditions,
    };
    for (const field of labelFields) {
      updatedConditions[field] = { env: ['foo'] };
    }
    standardRoleState.setRoleConditions(updatedConditions);

    if (appIdentityFields) {
      standardRoleState.markAppIdentityFieldsAsRequired(
        appIdentityFields,
        allPagesFetched
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return <>{children}</>;
};

export function DefineIdentities(props: StoryProps) {
  // Mock one possible empty state.
  if (!props.identitiesRequired) {
    return (
      <Provider key="no-identities">
        <AccessGraphDemoProvider>
          <WithAccessDefinition
            labelFields={['app_labels']}
            appIdentityFields={[]}
            allPagesFetched={true}
          >
            <MainComponent />
          </WithAccessDefinition>
        </AccessGraphDemoProvider>
      </Provider>
    );
  }

  const labelFields: LabelBasedResourceAccessFields[] = [];

  if (props.app) {
    labelFields.push('app_labels');
  }
  if (props.db) {
    labelFields.push('db_labels');
  }
  if (props.kubernetes) {
    labelFields.push('kubernetes_labels');
  }
  if (props.server) {
    labelFields.push('node_labels');
  }
  if (props.windowsDesktop) {
    labelFields.push('windows_desktop_labels');
  }

  if (labelFields.length === 0) {
    return <div>Select at least one access type from the story controls</div>;
  }

  return (
    <Provider key={labelFields.join('-')}>
      <AccessGraphDemoProvider>
        <WithAccessDefinition
          labelFields={labelFields}
          appIdentityFields={
            props.app
              ? [
                  'aws_role_arns',
                  'azure_identities',
                  'gcp_service_accounts',
                  'mcp',
                ]
              : undefined
          }
        >
          <MainComponent />
        </WithAccessDefinition>
      </AccessGraphDemoProvider>
    </Provider>
  );
}
