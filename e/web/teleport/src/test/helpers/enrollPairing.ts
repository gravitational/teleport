import { delay, http, HttpResponse } from 'msw';

import cfg from 'e-teleport/config';
import {
  CreateEnrollPairingResponse,
  GetEnrollPairingResponse,
} from 'e-teleport/services/devices';
import { JsonObject } from 'teleport/types';

export const createEnrollPairingSuccess = (
  mock: CreateEnrollPairingResponse = {
    state: 'awaiting_device',
    token: 'pairing-token',
    qrCode: 'base64-qr-code',
  }
) =>
  http.post(cfg.api.enrollPairing, () => {
    return HttpResponse.json(mock);
  });

export const createEnrollPairingError = (
  status: number,
  error: string | null = null,
  fields: JsonObject = {}
) =>
  http.post(cfg.api.enrollPairing, () => {
    return HttpResponse.json({ error: { message: error }, fields }, { status });
  });

export const createEnrollPairingForever = () =>
  http.post(cfg.api.enrollPairing, async () => await delay('infinite'));

/**
 * `getCurrentEnrollPairingSuccess` returns a handler for the polling endpoint
 * the wizard hits to follow the pairing's state machine. Override `mock.state`
 * to drive the wizard between steps:
 *
 * ```ts
 * // Hold the wizard on the QR step (default).
 * server.use(getCurrentEnrollPairingSuccess());
 *
 * // Advance the wizard past the QR step — the next poll returns
 * // awaiting_approval and the wizard transitions.
 * server.use(getCurrentEnrollPairingSuccess({
 *   state: 'awaiting_approval',
 *   token: 'pairing-token',
 * }));
 * ```
 *
 * Calling `server.use(...)` mid-test replaces the previous handler, so the
 * same wizard render can walk through the full state machine by re-registering
 * the handler with a new state between assertions.
 */
export const getCurrentEnrollPairingSuccess = (
  mock: GetEnrollPairingResponse = {
    state: 'awaiting_device',
    token: 'pairing-token',
  }
) =>
  http.get(cfg.api.enrollPairing, () => {
    return HttpResponse.json(mock);
  });

export const getCurrentEnrollPairingError = (
  status: number,
  error: string | null = null,
  fields: JsonObject = {}
) =>
  http.get(cfg.api.enrollPairing, () => {
    return HttpResponse.json({ error: { message: error }, fields }, { status });
  });
