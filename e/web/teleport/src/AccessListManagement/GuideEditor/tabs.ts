import {
  AmazonAws,
  Application,
  Database,
  Git,
  Kubernetes,
  Server,
  Windows,
} from 'design/Icon';
import { TabSpec } from 'design/SlideTabs/SlideTabs';

import {
  AppAccessInputFields,
  DatabaseAccessInputFields,
  GitHubOrganizationAccessInputFields,
  KubernetesAccessInputFields,
  ResourceAccessKind,
  ServerAccessInputFields,
  WindowsDesktopAccessInputFields,
} from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';

import { DefinableResourceAccessFields } from './Preset/role/listaccess';
import { RequiredAppIdentitiesWithFetchResult } from './Preset/role/resources/app';

export function getResourceAccessTabSpecs({
  idPrefix,
  accessKind,
  accessFields,
}: {
  idPrefix: string;
  /**
   * - identities: access related to resource identity (or principal)
   *   e.g.: db_names, db_users (not db_labels)
   * - access: anything related to resource access including identities
   *   e.g.: db_labels and db_names, etc.
   */
  accessKind: 'access' | 'identities';
  accessFields: DefinableResourceAccessFields[];
}): (TabSpec & {
  kind: ResourceAccessKind | 'awsIc';
  btnTitle: string;
  sectionTitle: string;
})[] {
  let titleTxt = '';
  switch (accessKind) {
    case 'access':
      titleTxt = 'Access';
      break;
    case 'identities':
      titleTxt = 'Identities';
      break;
    default:
      accessKind satisfies never;
  }

  return accessFields.map(field => {
    switch (field) {
      case 'app_labels': {
        const sectionTitle = `Application ${titleTxt}`;
        return {
          key: 'app',
          kind: 'app',
          ariaLabel: 'Go to Application tab',
          btnTitle: 'Application',
          icon: Application,
          controls: `${idPrefix}-app`,
          sectionTitle,
          tooltip: {
            content: sectionTitle,
          },
        };
      }
      case 'db_labels': {
        const sectionTitle = `Database ${titleTxt}`;
        return {
          key: 'db',
          ariaLabel: 'Go to Database tab',
          kind: 'db',
          btnTitle: 'Database',
          icon: Database,
          controls: `${idPrefix}-db`,
          sectionTitle,
          tooltip: {
            content: sectionTitle,
          },
        };
      }
      case 'kubernetes_labels': {
        const sectionTitle = `Kubernetes ${titleTxt}`;
        return {
          key: 'kube_cluster',
          ariaLabel: `"Go to Kubernetes Cluster tab"`,
          kind: 'kube_cluster',
          btnTitle: 'Kubernetes',
          icon: Kubernetes,
          controls: `${idPrefix}-kube_cluster`,
          sectionTitle,
          tooltip: {
            content: sectionTitle,
          },
        };
      }
      case 'node_labels': {
        const sectionTitle = `Server ${titleTxt}`;
        return {
          key: 'node',
          ariaLabel: 'Go to Server tab',
          kind: 'node',
          icon: Server,
          btnTitle: 'Server',
          controls: `${idPrefix}-node`,
          sectionTitle,
          tooltip: {
            content: sectionTitle,
          },
        };
      }
      case 'windows_desktop_labels': {
        const sectionTitle = `Windows Desktop ${titleTxt}`;
        return {
          key: 'windows_desktop',
          ariaLabel: 'Go to Windows Desktop tab',
          kind: 'windows_desktop',
          icon: Windows,
          btnTitle: 'Desktops',
          controls: `${idPrefix}-windows_desktop`,
          sectionTitle,
          tooltip: {
            content: sectionTitle,
          },
        };
      }
      case 'github_permissions': {
        const sectionTitle = `Git Server ${titleTxt}`;
        return {
          key: 'github_permissions',
          ariaLabel: 'Go to Git Server tab',
          kind: 'git_server',
          icon: Git,
          btnTitle: 'Git Servers',
          controls: `${idPrefix}-git_server`,
          sectionTitle,
          tooltip: {
            content: sectionTitle,
          },
        };
      }
      case 'awsIc': {
        const sectionTitle = `AWS Identity Center ${titleTxt}`;
        return {
          key: 'awsIc',
          ariaLabel: 'Go to AWS Identity tab',
          kind: 'awsIc',
          icon: AmazonAws,
          btnTitle: 'AWS Identity Center',
          controls: `${idPrefix}-awsIc`,
          sectionTitle,
          tooltip: {
            content: sectionTitle,
          },
        };
      }
      default:
        field satisfies never;
    }
  });
}

export function getRoleSectionInputFieldConfig({
  kind,
  requiredAppIdentities,
  withLabels,
}: {
  kind: ResourceAccessKind;
  requiredAppIdentities: RequiredAppIdentitiesWithFetchResult;
  withLabels: boolean;
}):
  | AppAccessInputFields
  | DatabaseAccessInputFields
  | KubernetesAccessInputFields
  | ServerAccessInputFields
  | WindowsDesktopAccessInputFields
  | GitHubOrganizationAccessInputFields {
  switch (kind) {
    case 'app': {
      return {
        labels: withLabels,
        awsRoleARNs: !!requiredAppIdentities.aws_role_arns,
        azureIdentities: !!requiredAppIdentities.azure_identities,
        gcpServiceAccounts: !!requiredAppIdentities.gcp_service_accounts,
        mcpTools: !!requiredAppIdentities.mcp?.tools,
      };
    }

    case 'db': {
      return {
        labels: withLabels,
        dbServiceLabels: false,
        roles: false,
        names: true,
        users: true,
      };
    }

    case 'kube_cluster': {
      return {
        labels: withLabels,
        resources: true,
        users: true,
        groups: true,
      };
    }
    case 'node': {
      return {
        labels: withLabels,
        logins: true,
      };
    }
    case 'windows_desktop': {
      return {
        labels: withLabels,
        logins: true,
      };
    }
    case 'git_server':
      return {
        organizations: true,
      };
    default:
      kind satisfies never;
  }
}
