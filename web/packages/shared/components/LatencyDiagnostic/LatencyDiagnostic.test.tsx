/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  latencyColors,
  ERROR_THRESHOLD,
  LatencyColor,
  WARN_THRESHOLD,
} from 'shared/components/LatencyDiagnostic/LatencyDiagnostic';

test('latency colors', () => {
  // unknown
  // green + green = green
  expect(latencyColors(undefined)).toStrictEqual({
    client: LatencyColor.Unknown,
    server: LatencyColor.Unknown,
    total: LatencyColor.Unknown,
  });

  // green + green = green
  expect(
    latencyColors({ client: WARN_THRESHOLD - 1, server: WARN_THRESHOLD - 1 })
  ).toStrictEqual({
    client: LatencyColor.Ok,
    server: LatencyColor.Ok,
    total: LatencyColor.Ok,
  });

  // green + yellow = yellow
  expect(
    latencyColors({ client: WARN_THRESHOLD - 1, server: WARN_THRESHOLD })
  ).toStrictEqual({
    client: LatencyColor.Ok,
    server: LatencyColor.Warn,
    total: LatencyColor.Warn,
  });
  expect(
    latencyColors({ client: WARN_THRESHOLD, server: WARN_THRESHOLD - 1 })
  ).toStrictEqual({
    client: LatencyColor.Warn,
    server: LatencyColor.Ok,
    total: LatencyColor.Warn,
  });

  // green + red = red
  expect(
    latencyColors({ client: WARN_THRESHOLD - 1, server: ERROR_THRESHOLD })
  ).toStrictEqual({
    client: LatencyColor.Ok,
    server: LatencyColor.Error,
    total: LatencyColor.Error,
  });
  expect(
    latencyColors({ client: ERROR_THRESHOLD, server: WARN_THRESHOLD - 1 })
  ).toStrictEqual({
    client: LatencyColor.Error,
    server: LatencyColor.Ok,
    total: LatencyColor.Error,
  });

  // yellow + yellow = yellow
  expect(
    latencyColors({ client: WARN_THRESHOLD, server: WARN_THRESHOLD })
  ).toStrictEqual({
    client: LatencyColor.Warn,
    server: LatencyColor.Warn,
    total: LatencyColor.Warn,
  });

  // yellow + red = red
  expect(
    latencyColors({ client: WARN_THRESHOLD, server: ERROR_THRESHOLD })
  ).toStrictEqual({
    client: LatencyColor.Warn,
    server: LatencyColor.Error,
    total: LatencyColor.Error,
  });
  expect(
    latencyColors({ client: ERROR_THRESHOLD, server: WARN_THRESHOLD })
  ).toStrictEqual({
    client: LatencyColor.Error,
    server: LatencyColor.Warn,
    total: LatencyColor.Error,
  });

  // red + red = red
  expect(
    latencyColors({ client: ERROR_THRESHOLD, server: ERROR_THRESHOLD })
  ).toStrictEqual({
    client: LatencyColor.Error,
    server: LatencyColor.Error,
    total: LatencyColor.Error,
  });
});
