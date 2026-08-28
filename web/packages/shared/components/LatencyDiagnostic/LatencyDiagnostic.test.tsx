/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import {
  ERROR_THRESHOLD,
  LatencyColor,
  latencyColors,
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
