/**
 * @jest-environment node
 */
/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { EventEmitter } from 'node:events';

import { Metadata, ServerUnaryCall } from '@grpc/grpc-js';

import {
  PromptHardwareKeyTouchRequest,
  PromptHardwareKeyTouchResponse,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

import Logger, { NullService } from 'teleterm/logger';
import { TshdEventContextBridgeService } from 'teleterm/types';

import { createService } from './index';

beforeAll(() => {
  Logger.init(new NullService());
});

test.each([
  { timing: 'before', callbacksAfterSubscription: 1 },
  { timing: 'after', callbacksAfterSubscription: 0 },
])(
  'delivers cancellation that arrives $timing callback subscription',
  ({ timing, callbacksAfterSubscription }) => {
    const { service, setupTshdEventContextBridgeService } = createService(
      new Logger('tshd events')
    );
    const call = new MockServerUnaryCall<
      PromptHardwareKeyTouchRequest,
      PromptHardwareKeyTouchResponse
    >(
      PromptHardwareKeyTouchRequest.create({
        proxyHostname: 'proxy.example.com',
      })
    );
    let onRequestCancelled!: (callback: () => void) => void;
    setupTshdEventContextBridgeService(
      createMockTshdEventContextBridgeService({
        promptHardwareKeyTouch: async args => {
          onRequestCancelled = args.onRequestCancelled;
          return {};
        },
      })
    );
    service.promptHardwareKeyTouch(call, jest.fn());

    const cancel = () => {
      call.cancelled = true;
      call.emit('cancelled');
    };
    const firstCallback = jest.fn();
    const secondCallback = jest.fn();

    if (timing === 'before') {
      cancel();
    }
    onRequestCancelled(firstCallback);
    onRequestCancelled(secondCallback);
    expect(firstCallback).toHaveBeenCalledTimes(callbacksAfterSubscription);
    expect(secondCallback).toHaveBeenCalledTimes(callbacksAfterSubscription);
    if (timing === 'after') {
      cancel();
    }

    expect(firstCallback).toHaveBeenCalledTimes(1);
    expect(secondCallback).toHaveBeenCalledTimes(1);
  }
);

class MockServerUnaryCall<Request, Response>
  extends EventEmitter
  implements ServerUnaryCall<Request, Response>
{
  cancelled = false;
  readonly metadata = new Metadata();

  constructor(public request: Request) {
    super();
  }

  getPeer = unexpectedCall;
  sendMetadata = unexpectedCall;
  getDeadline = unexpectedCall;
  getPath = unexpectedCall;
  getHost = unexpectedCall;
  getAuthContext = unexpectedCall;
  getMetricsRecorder = unexpectedCall;
}

function createMockTshdEventContextBridgeService(
  props: Partial<TshdEventContextBridgeService>
): TshdEventContextBridgeService {
  return {
    relogin: unexpectedCall,
    promptHardwareKeyPIN: unexpectedCall,
    promptHardwareKeyPINChange: unexpectedCall,
    promptHardwareKeyTouch: unexpectedCall,
    confirmHardwareKeySlotOverwrite: unexpectedCall,
    sendNotification: unexpectedCall,
    sendPendingHeadlessAuthentication: unexpectedCall,
    promptMFA: unexpectedCall,
    getUsageReportingSettings: unexpectedCall,
    reportUnexpectedVnetShutdown: unexpectedCall,
    reportVnetSessionExpired: unexpectedCall,
    ...props,
  };
}

function unexpectedCall(): never {
  throw new Error('Unexpected mock method call');
}
