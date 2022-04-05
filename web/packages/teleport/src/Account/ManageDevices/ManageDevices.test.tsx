/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { render, fireEvent, waitFor, screen } from 'design/utils/testing';
import { Context, ContextProvider } from 'teleport';
import authService from 'teleport/services/auth';
import ManageDevices from './ManageDevices';
import cfg from 'teleport/config';

const privilegeToken = 'privilegeToken123';
const restrictedPrivilegeToken = 'restrictedPrivilegeToken123';

describe('mfa device dashboard testing', () => {
  let renderManageDevices;
  let ctx: Context;

  beforeEach(() => {
    ctx = new Context();

    jest.spyOn(ctx.mfaService, 'fetchDevices').mockResolvedValue([
      {
        id: '1',
        description: 'Authenticator App',
        name: 'iphone 12',
        registeredDate: new Date(1628799417000),
        lastUsedDate: new Date(1628799417000),
      },
      {
        id: '2',
        description: 'Hardware Key',
        name: 'yubikey',
        registeredDate: new Date(1623722252000),
        lastUsedDate: new Date(1623981452000),
      },
    ]);

    jest
      .spyOn(authService, 'createMfaRegistrationChallenge')
      .mockResolvedValue({
        qrCode: '123456',
        webauthnPublicKey: null,
      });

    jest.spyOn(ctx.mfaService, 'addNewTotpDevice').mockResolvedValue({});

    jest.spyOn(ctx.mfaService, 'addNewWebauthnDevice').mockResolvedValue({});

    jest.spyOn(ctx.mfaService, 'removeDevice').mockResolvedValue({});

    renderManageDevices = () =>
      render(
        <ContextProvider ctx={ctx}>
          <ManageDevices />
        </ContextProvider>
      );

    jest.spyOn(cfg, 'getAuth2faType').mockReturnValue('optional');

    jest.spyOn(authService, 'checkWebauthnSupport').mockResolvedValue();

    jest
      .spyOn(authService, 'createPrivilegeTokenWithTotp')
      .mockResolvedValue(privilegeToken);

    jest
      .spyOn(authService, 'createPrivilegeTokenWithWebauthn')
      .mockResolvedValue(privilegeToken);

    jest
      .spyOn(authService, 'createRestrictedPrivilegeToken')
      .mockResolvedValue(restrictedPrivilegeToken);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('re-authenticating with totp and adding a webauthn device', async () => {
    await waitFor(() => renderManageDevices());

    fireEvent.click(screen.getByText(/add two-factor device/i));

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    const reAuthMfaSelectEl = screen
      .getByTestId('mfa-select')
      .querySelector('input');
    fireEvent.keyDown(reAuthMfaSelectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getAllByText(/authenticator app/i)[1]);

    const tokenField = screen.getByPlaceholderText('123 456');
    fireEvent.change(tokenField, { target: { value: '321321' } });

    await waitFor(() => {
      fireEvent.click(screen.getByText('Continue'));
    });

    expect(authService.createPrivilegeTokenWithTotp).toHaveBeenCalledWith(
      '321321'
    );

    expect(screen.getByText('Add New Two-Factor Device')).toBeInTheDocument();

    const deviceNameField = screen.getByPlaceholderText('Name');
    fireEvent.change(deviceNameField, { target: { value: 'yubikey' } });

    await waitFor(() => {
      fireEvent.click(screen.getByText('Add device'));
    });

    expect(ctx.mfaService.addNewWebauthnDevice).toHaveBeenCalledWith(
      expect.objectContaining({
        tokenId: privilegeToken,
        deviceName: 'yubikey',
      })
    );
  });

  test('re-authenticating with webauthn and adding a totp device', async () => {
    jest.spyOn(cfg, 'getPreferredMfaType').mockReturnValue('webauthn');
    await waitFor(() => renderManageDevices());

    fireEvent.click(screen.getByText(/add two-factor device/i));

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    await waitFor(() => {
      fireEvent.click(screen.getByText('Continue'));
    });

    expect(authService.createPrivilegeTokenWithWebauthn).toHaveBeenCalled();

    expect(screen.getByText('Add New Two-Factor Device')).toBeInTheDocument();

    const addDeviceMfaSelectEl = screen
      .getByTestId('mfa-select')
      .querySelector('input');
    fireEvent.keyDown(addDeviceMfaSelectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getAllByText(/authenticator app/i)[1]);

    const addDeviceTokenField = screen.getByPlaceholderText('123 456');
    fireEvent.change(addDeviceTokenField, { target: { value: '321321' } });

    const deviceNameField = screen.getByPlaceholderText('Name');
    fireEvent.change(deviceNameField, { target: { value: 'iphone 12' } });

    await waitFor(() => {
      fireEvent.click(screen.getByText('Add device'));
    });

    expect(ctx.mfaService.addNewTotpDevice).toHaveBeenCalledWith(
      expect.objectContaining({
        tokenId: privilegeToken,
        deviceName: 'iphone 12',
        secondFactorToken: '321321',
      })
    );
  });

  test('adding a first device', async () => {
    jest.spyOn(ctx.mfaService, 'fetchDevices').mockResolvedValue([]);

    await waitFor(() => renderManageDevices());

    await waitFor(() =>
      fireEvent.click(screen.getByText(/add two-factor device/i))
    );

    expect(authService.createRestrictedPrivilegeToken).toHaveBeenCalled();

    expect(screen.getByText('Add New Two-Factor Device')).toBeInTheDocument();

    const deviceNameField = screen.getByPlaceholderText('Name');
    fireEvent.change(deviceNameField, { target: { value: 'yubikey' } });

    await waitFor(() => {
      fireEvent.click(screen.getByText('Add device'));
    });

    expect(ctx.mfaService.addNewWebauthnDevice).toHaveBeenCalledWith(
      expect.objectContaining({
        tokenId: restrictedPrivilegeToken,
        deviceName: 'yubikey',
      })
    );
  });

  test('removing a device', async () => {
    await waitFor(() => renderManageDevices());

    fireEvent.click(screen.getAllByText(/remove/i)[0]);

    expect(screen.getByText('Verify your identity')).toBeInTheDocument();

    await waitFor(() => {
      fireEvent.click(screen.getByText('Continue'));
    });

    expect(authService.createPrivilegeTokenWithWebauthn).toHaveBeenCalled();

    expect(
      screen.getByText(/Are you sure you want to remove device/i)
    ).toBeInTheDocument();

    await waitFor(() => {
      fireEvent.click(screen.getAllByText('Remove')[2]);
    });

    expect(ctx.mfaService.removeDevice).toHaveBeenCalledWith(
      privilegeToken,
      'iphone 12'
    );
  });
});
