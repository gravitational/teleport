import cfg from 'e-teleport/config';
import { UrlResourcesParams } from 'teleport/config';
import { TrustedDevice } from 'teleport/DeviceTrust/types';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import { makeDevices } from './makeDevices';

type EnrollPairingState = 'awaiting_device' | 'awaiting_approval' | 'approved';

export type CreateEnrollPairingResponse = {
  state: EnrollPairingState;
  token: string;
  // base64-encoded PNG. Only populated when state is "awaiting_device".
  qrCode?: string;
};

export type GetEnrollPairingResponse = {
  state: EnrollPairingState;
  token: string;
  // The device asking to enroll. Populated once the mobile app has claimed the pairing through
  // CreatePairedDeviceEnrollToken, so from "awaiting_approval" onward.
  device?: EnrollPairingDevice;
};

type EnrollPairingDevice = {
  // Friendly OS name, e.g. "iOS".
  osType: string;
  serialNumber: string;
  // OS version without the leading 'v', e.g. "26.3.1".
  osVersion: string;
};

export const deviceService = {
  fetchDevices(params?: UrlResourcesParams) {
    return api.get(cfg.getTrustedDevicesUrl(params)).then(makeDevices);
  },
  /**
   * @remarks
   * The declared return type does not match the wire shape. The endpoint returns the devices under
   * `items`, but `ResourcesResponse` types the array as `agents`. Callers compensate with `dataKey:
   * 'items'` in useKeyBasedPagination and tests mocking the real shape need ts-expect-error.
   */
  fetchDevicesByUser(
    params: UrlResourcesParams,
    signal: AbortSignal
  ): Promise<ResourcesResponse<TrustedDevice>> {
    // TODO(ravicious): Fix types for fetchDevicesByUser as described in the remarks section.
    return api.get(cfg.getTrustedDevicesByUserUrl(params), signal);
  },
  createEnrollPairing(): Promise<CreateEnrollPairingResponse> {
    return api.post(cfg.getEnrollPairingUrl());
  },
  getCurrentEnrollPairing(): Promise<GetEnrollPairingResponse> {
    return api.get(cfg.getEnrollPairingUrl());
  },
  approveEnrollPairing(token: string): Promise<void> {
    return api.post(cfg.getEnrollPairingApproveUrl(), { token });
  },
  denyEnrollPairing(token: string): Promise<void> {
    return api.post(cfg.getEnrollPairingDenyUrl(), { token });
  },
};
