/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useState } from 'react';
import { GitServer } from 'web/packages/teleport/src/services/gitServers';

import { ButtonBorder, ButtonWithMenu, MenuItem } from 'design';
import { AwsLaunchButton } from 'shared/components/AwsLaunchButton';
import {
  LoginItem,
  MenuInputType,
  MenuLogin,
} from 'shared/components/MenuLogin';
import { AwsRole } from 'shared/services/apps';

import { TcpAppConnectDialog } from 'teleport/Apps/TcpAppConnectDialog';
import cfg from 'teleport/config';
import DbConnectDialog from 'teleport/Databases/ConnectDialog';
import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';
import { ResourceKind } from 'teleport/Discover/Shared';
import { ConnectDialog as GitServerConnectDialog } from 'teleport/GitServers';
import KubeConnectDialog from 'teleport/Kubes/ConnectDialog';
import { openNewTab } from 'teleport/lib/util';
import { useSamlAppAction } from 'teleport/SamlApplications/useSamlAppActions';
import { UnifiedResource } from 'teleport/services/agents';
import { App, AppSubKind } from 'teleport/services/apps';
import { Database } from 'teleport/services/databases';
import { Desktop } from 'teleport/services/desktops';
import { Kube } from 'teleport/services/kube';
import { Node, sortNodeLogins } from 'teleport/services/nodes';
import { DiscoverEventResource } from 'teleport/services/userEvent';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

type Props = {
  resource: UnifiedResource;
};

export const ResourceActionButton = ({ resource }: Props) => {
  switch (resource.kind) {
    case 'node':
      return <NodeConnect node={resource} />;
    case 'app':
      return <AppLaunch app={resource} />;
    case 'db':
      return <DatabaseConnect database={resource} />;
    case 'kube_cluster':
      return <KubeConnect kube={resource} />;
    case 'windows_desktop':
      return <DesktopConnect desktop={resource} />;
    case 'git_server':
      return <GitServerConnect gitServer={resource} />;
    default:
      return null;
  }
};

const NodeConnect = ({ node }: { node: Node }) => {
  const { clusterId } = useStickyClusterId();
  const startSshSession = (login: string, serverId: string) => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId,
      login,
    });

    openNewTab(url);
  };

  function handleOnOpen() {
    return makeNodeOptions(clusterId, node);
  }

  const handleOnSelect = (e: React.SyntheticEvent, login: string) => {
    e.preventDefault();
    return startSshSession(login, node.id);
  };

  return (
    <MenuLogin
      width="123px"
      inputType={MenuInputType.FILTER}
      textTransform={'none'}
      alignButtonWidthToMenu
      getLoginItems={handleOnOpen}
      onSelect={handleOnSelect}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      anchorOrigin={{
        vertical: 'bottom',
        horizontal: 'right',
      }}
    />
  );
};

const DesktopConnect = ({ desktop }: { desktop: Desktop }) => {
  const { clusterId } = useStickyClusterId();
  const startRemoteDesktopSession = (username: string, desktopName: string) => {
    const url = cfg.getDesktopRoute({
      clusterId,
      desktopName,
      username,
    });

    openNewTab(url);
  };

  function handleOnOpen() {
    return makeDesktopLoginOptions(clusterId, desktop.name, desktop.logins);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    e.preventDefault();
    return startRemoteDesktopSession(login, desktop.name);
  }

  return (
    <MenuLogin
      width="123px"
      inputType={MenuInputType.FILTER}
      textTransform="none"
      alignButtonWidthToMenu
      getLoginItems={handleOnOpen}
      onSelect={handleOnSelect}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      anchorOrigin={{
        vertical: 'bottom',
        horizontal: 'right',
      }}
    />
  );
};

type AppLaunchProps = {
  app: App;
};
const AppLaunch = ({ app }: AppLaunchProps) => {
  const {
    name,
    launchUrl,
    awsConsole,
    awsRoles,
    fqdn,
    clusterId,
    publicAddr,
    isCloud,
    isTcp,
    samlApp,
    samlAppSsoUrl,
    samlAppPreset,
    subKind,
    permissionSets,
  } = app;
  const { actions, userSamlIdPPerm } = useSamlAppAction();

  const isAwsIdentityCenterApp = subKind === AppSubKind.AwsIcAccount;
  function getAwsLaunchUrl(arnOrPermSetName: string) {
    if (isAwsIdentityCenterApp) {
      return `${publicAddr}&role_name=${arnOrPermSetName}`;
    } else {
      return cfg.getAppLauncherRoute({
        fqdn,
        clusterId,
        publicAddr,
        arn: arnOrPermSetName,
      });
    }
  }
  if (awsConsole || isAwsIdentityCenterApp) {
    let awsConsoleOrIdentityCenterRoles: AwsRole[] = awsRoles;
    if (isAwsIdentityCenterApp) {
      awsConsoleOrIdentityCenterRoles = permissionSets.map(
        (ps): AwsRole => ({
          name: ps.name,
          arn: ps.name,
          display: ps.name,
          accountId: name,
        })
      );
    }

    return (
      <AwsLaunchButton
        width="123px"
        awsRoles={awsConsoleOrIdentityCenterRoles}
        getLaunchUrl={getAwsLaunchUrl}
        isAwsIdentityCenterApp={isAwsIdentityCenterApp}
      />
    );
  }
  if (isCloud) {
    return (
      <ButtonBorder
        disabled
        width="123px"
        size="small"
        title="Cloud apps cannot be launched by the browser"
        textTransform="none"
      >
        Launch
      </ButtonBorder>
    );
  }
  if (isTcp) {
    return <TcpAppConnect app={app} />;
  }
  if (samlApp) {
    if (actions.showActions) {
      const currentSamlAppSpec: ResourceSpec = {
        name: name,
        event: DiscoverEventResource.SamlApplication,
        kind: ResourceKind.SamlApplication,
        samlMeta: { preset: samlAppPreset },
        icon: 'application',
        keywords: ['saml'],
      };
      return (
        <ButtonWithMenu
          text="Log In"
          width="123px"
          size="small"
          target="_blank"
          href={samlAppSsoUrl}
          rel="noreferrer"
          textTransform="none"
          forwardedAs="a"
          title="Log in to SAML application"
        >
          <MenuItem
            onClick={() => actions.startEdit(currentSamlAppSpec)}
            disabled={!userSamlIdPPerm.edit} // disable props does not disable onClick
          >
            Edit
          </MenuItem>
          <MenuItem
            onClick={() => actions.startDelete(currentSamlAppSpec)}
            disabled={!userSamlIdPPerm.remove} // disable props does not disable onClick
          >
            Delete
          </MenuItem>
        </ButtonWithMenu>
      );
    } else {
      return (
        <ButtonBorder
          as="a"
          width="123px"
          size="small"
          target="_blank"
          href={samlAppSsoUrl}
          rel="noreferrer"
          textTransform="none"
          title="Log in to SAML application"
        >
          Log In
        </ButtonBorder>
      );
    }
  }
  return (
    <ButtonBorder
      as="a"
      width="123px"
      size="small"
      target="_blank"
      href={launchUrl}
      rel="noreferrer"
      textTransform="none"
    >
      Launch
    </ButtonBorder>
  );
};

function DatabaseConnect({ database }: { database: Database }) {
  const { name, protocol, supportsInteractive } = database;
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const [open, setOpen] = useState(false);
  const username = ctx.storeUser.state.username;
  const authType = ctx.storeUser.state.authType;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  return (
    <>
      <ButtonBorder
        textTransform="none"
        width="123px"
        size="small"
        onClick={() => {
          setOpen(true);
        }}
      >
        Connect
      </ButtonBorder>
      {open && (
        <DbConnectDialog
          username={username}
          clusterId={clusterId}
          dbName={name}
          dbProtocol={protocol}
          onClose={() => setOpen(false)}
          authType={authType}
          accessRequestId={accessRequestId}
          supportsInteractive={supportsInteractive}
        />
      )}
    </>
  );
}

const KubeConnect = ({ kube }: { kube: Kube }) => {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const [open, setOpen] = useState(false);
  const username = ctx.storeUser.state.username;
  const authType = ctx.storeUser.state.authType;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  return (
    <>
      <ButtonBorder
        width="123px"
        textTransform="none"
        size="small"
        onClick={() => setOpen(true)}
      >
        Connect
      </ButtonBorder>
      {open && (
        <KubeConnectDialog
          onClose={() => setOpen(false)}
          username={username}
          authType={authType}
          kubeConnectName={kube.name}
          clusterId={clusterId}
          accessRequestId={accessRequestId}
        />
      )}
    </>
  );
};

function GitServerConnect({ gitServer }: { gitServer: GitServer }) {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const [open, setOpen] = useState(false);
  const organization = gitServer.github.organization;
  const username = ctx.storeUser.state.username;
  const authType = ctx.storeUser.state.authType;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  return (
    <>
      <ButtonBorder
        textTransform="none"
        width="123px"
        size="small"
        onClick={() => {
          setOpen(true);
        }}
      >
        Connect
      </ButtonBorder>
      {open && (
        <GitServerConnectDialog
          username={username}
          clusterId={clusterId}
          organization={organization}
          onClose={() => setOpen(false)}
          authType={authType}
          accessRequestId={accessRequestId}
        />
      )}
    </>
  );
}

function TcpAppConnect({ app }: { app: App }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <ButtonBorder
        textTransform="none"
        width="123px"
        size="small"
        onClick={() => setOpen(true)}
      >
        Connect
      </ButtonBorder>
      {open && <TcpAppConnectDialog app={app} onClose={() => setOpen(false)} />}
    </>
  );
}

const makeNodeOptions = (clusterId: string, node: Node | undefined) => {
  const nodeLogins = node?.sshLogins || [];
  const logins = sortNodeLogins(nodeLogins);

  return logins.map(login => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId: node?.id || '',
      login,
    });

    return {
      login,
      url,
    };
  });
};

const makeDesktopLoginOptions = (
  clusterId: string,
  desktopName = '',
  logins = [] as string[]
): LoginItem[] => {
  return logins.map(username => {
    const url = cfg.getDesktopRoute({
      clusterId,
      desktopName,
      username,
    });

    return {
      login: username,
      url,
    };
  });
};
