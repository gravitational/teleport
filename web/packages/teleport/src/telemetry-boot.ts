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

import { ZoneContextManager } from '@opentelemetry/context-zone';
import {
  CompositePropagator,
  W3CTraceContextPropagator,
} from '@opentelemetry/core';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';
import { B3Propagator } from '@opentelemetry/propagator-b3';
import {
  defaultResource,
  resourceFromAttributes,
} from '@opentelemetry/resources';
import {
  BatchSpanProcessor,
  ConsoleSpanExporter,
} from '@opentelemetry/sdk-trace-base';
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
} from '@opentelemetry/semantic-conventions';

import { getAuthHeaders } from 'teleport/services/api/api';

export function instantiateTelemetry() {
  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new UserInteractionInstrumentation(),
      new XMLHttpRequestInstrumentation(),
      new FetchInstrumentation(),
    ],
  });

  const resource = defaultResource().merge(
    resourceFromAttributes({
      [ATTR_SERVICE_NAME]: 'teleport-web-ui',
      [ATTR_SERVICE_VERSION]: '0.1.0',
    })
  );

  const provider = new WebTracerProvider({
    resource: resource,
    spanProcessors: [
      new BatchSpanProcessor(new ConsoleSpanExporter()),
      new BatchSpanProcessor(
        new OTLPTraceExporter({
          timeoutMillis: 15000,
          url: `${window.location.origin}/v1/webapi/traces`,
          concurrencyLimit: 10, // an optional limit on pending requests
          headers: async () => getAuthHeaders(),
        })
      ),
    ],
  });

  provider.register({
    contextManager: new ZoneContextManager(),
    propagator: new CompositePropagator({
      propagators: [new B3Propagator(), new W3CTraceContextPropagator()],
    }),
  });
}
