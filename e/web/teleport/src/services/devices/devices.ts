import cfg from 'e-teleport/config';
import { UrlResourcesParams } from 'teleport/config';
import { TrustedDevice } from 'teleport/DeviceTrust/types';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import { makeDevices } from './makeDevices';

export type EnrollPairingState =
  | 'awaiting_device'
  | 'awaiting_approval'
  | 'approved';

export type CreateEnrollPairingResponse = {
  state: EnrollPairingState;
  token: string;
  // base64-encoded PNG. Only populated when state is "awaiting_device".
  qrCode?: string;
};

export type GetEnrollPairingResponse = {
  state: EnrollPairingState;
  token: string;
};

export const deviceService = {
  fetchDevices(params?: UrlResourcesParams) {
    return api.get(cfg.getTrustedDevicesUrl(params)).then(makeDevices);
  },
  fetchDevicesByUser(
    params: UrlResourcesParams,
    signal: AbortSignal
  ): Promise<ResourcesResponse<TrustedDevice>> {
    return api.get(cfg.getTrustedDevicesByUserUrl(params), signal);
  },
  createEnrollPairing(): Promise<CreateEnrollPairingResponse> {
    return api.post(cfg.getEnrollPairingUrl());
  },
  getCurrentEnrollPairing(): Promise<GetEnrollPairingResponse> {
    return api.get(cfg.getEnrollPairingUrl());
  },
};
