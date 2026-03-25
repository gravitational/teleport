/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { fireEvent, render, screen } from 'design/utils/testing';

import cfg from 'teleport/config';
import {
  createTeleportContext,
  getAcl,
  noAccess,
} from 'teleport/mocks/contexts';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { RoleDiffProps, RoleDiffState } from '../Roles';
import { RoleEditorVisualizer } from './RoleEditorVisualizer';

const defaultDemoMode = cfg.entitlements.AccessGraphDemoMode;
const defaultIsPolicyEnabled = cfg.isPolicyEnabled;
const defaultIsCloud = cfg.isCloud;
afterEach(() => {
  cfg.entitlements.AccessGraphDemoMode = defaultDemoMode;
  cfg.isPolicyEnabled = defaultIsPolicyEnabled;
  cfg.isCloud = defaultIsCloud;
});

const roleDiffElement = <div>i am rendered</div>;
test('no roleDiffProps shows policyPlaceHolder', async () => {
  render(getComponent(makeRoleDiffProps()));
  expect(
    screen.getByText('Teleport Identity Security saves you from mistakes.')
  ).toBeInTheDocument();
});

test('POLICY_ENABLED displays roll diff visualizer', async () => {
  render(
    getComponent(
      makeRoleDiffProps({ roleDiffState: RoleDiffState.PolicyEnabled })
    )
  );
  expect(screen.getByText('i am rendered')).toBeInTheDocument();
});

test('Preview Identity Security button does not show for non-cloud', async () => {
  cfg.isCloud = false;
  render(
    getComponent(
      makeRoleDiffProps({
        roleDiffState: RoleDiffState.Disabled,
      })
    )
  );
  expect(
    screen.queryByText('Preview Identity Security')
  ).not.toBeInTheDocument();
});

test('Preview Identity Security button does not show if entitlement not enabled', async () => {
  cfg.isCloud = true;
  render(
    getComponent(
      makeRoleDiffProps({
        roleDiffState: RoleDiffState.Disabled,
      })
    )
  );
  expect(
    screen.queryByText('Preview Identity Security')
  ).not.toBeInTheDocument();
});

test('Preview Identity Security button displays for cloud users with entitlement', async () => {
  cfg.isCloud = true;
  cfg.entitlements.AccessGraphDemoMode = { enabled: true, limit: 0 };
  render(
    getComponent(
      makeRoleDiffProps({
        roleDiffState: RoleDiffState.Disabled,
      })
    )
  );
  expect(screen.getByText('Preview Identity Security')).toBeInTheDocument();
});

test('Preview Identity Security button does not show if user does not have update ACL', async () => {
  cfg.isCloud = true;
  cfg.entitlements.AccessGraphDemoMode = { enabled: true, limit: 0 };
  const ctx = createTeleportContext({
    customAcl: { ...getAcl(), accessGraphSettings: noAccess },
  });
  render(
    getComponent(
      makeRoleDiffProps({
        roleDiffState: RoleDiffState.Disabled,
      }),
      ctx
    )
  );
  expect(
    screen.queryByText('Preview Identity Security')
  ).not.toBeInTheDocument();
});

test('DEMO_READY displays roll diff visualizer with demo banner', async () => {
  render(
    getComponent(makeRoleDiffProps({ roleDiffState: RoleDiffState.DemoReady }))
  );
  expect(screen.getByTestId('demo-banner')).toBeInTheDocument();
  expect(screen.getByText('Contact Sales')).toBeInTheDocument();
  expect(screen.getByText('Learn More')).toBeInTheDocument();
});

test('demo banner is dismissed and shows again on rerender', async () => {
  const { rerender } = render(
    getComponent(makeRoleDiffProps({ roleDiffState: RoleDiffState.DemoReady }))
  );
  expect(screen.getByTestId('demo-banner')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
  expect(screen.queryByText('demo-banner')).not.toBeInTheDocument();
  rerender(
    getComponent(makeRoleDiffProps({ roleDiffState: RoleDiffState.DemoReady }))
  );
  expect(screen.getByTestId('demo-banner')).toBeInTheDocument();
});

test('ERROR displays policy placeholder with error', async () => {
  const errorMessage = 'i am an error';
  render(
    getComponent(
      makeRoleDiffProps({
        roleDiffState: RoleDiffState.Error,
        roleDiffErrorMessage: errorMessage,
      })
    )
  );
  expect(screen.getByText(errorMessage)).toBeInTheDocument();
});

test('LOADING_SETTINGS displays policy placeholder with a preview button in a loading state', async () => {
  cfg.isCloud = true;
  cfg.entitlements.AccessGraphDemoMode = { enabled: true, limit: 0 };
  render(
    getComponent(
      makeRoleDiffProps({ roleDiffState: RoleDiffState.LoadingSettings })
    )
  );
  expect(screen.getByText('Creating graph…')).toBeInTheDocument();
});

test('WAITING_FOR_SYNC displays policy placeholder with a preview button in a loading state', async () => {
  cfg.isCloud = true;
  cfg.entitlements.AccessGraphDemoMode = { enabled: true, limit: 0 };
  render(
    getComponent(
      makeRoleDiffProps({ roleDiffState: RoleDiffState.WaitingForSync })
    )
  );
  expect(screen.getByText('Creating graph…')).toBeInTheDocument();
});

function makeRoleDiffProps(props?: Partial<RoleDiffProps>) {
  return {
    roleDiffElement,
    updateRoleDiff: () => null,
    enableDemoMode: () => null,
    ...props,
  };
}

function getComponent(props: RoleDiffProps, customCtx?: TeleportContext) {
  const ctx = customCtx || createTeleportContext();
  return (
    <TeleportContextProvider ctx={ctx}>
      <RoleEditorVisualizer currentFlow="creating" roleDiffProps={props} />
    </TeleportContextProvider>
  );
}
