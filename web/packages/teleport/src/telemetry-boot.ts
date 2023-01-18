import { Resource } from '@opentelemetry/resources';
import { SemanticResourceAttributes } from '@opentelemetry/semantic-conventions';
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import {
  ConsoleSpanExporter,
  BatchSpanProcessor,
} from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { B3Propagator } from '@opentelemetry/propagator-b3';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';
import {
  CompositePropagator,
  W3CTraceContextPropagator,
} from '@opentelemetry/core';
import { ZoneContextManager } from '@opentelemetry/context-zone';

export function instantiateTelemetry() {
  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new UserInteractionInstrumentation(),
      new XMLHttpRequestInstrumentation(),
      new FetchInstrumentation(),
    ],
  });

  const resource = Resource.default().merge(
    new Resource({
      [SemanticResourceAttributes.SERVICE_NAME]: 'teleport-web-ui',
      [SemanticResourceAttributes.SERVICE_VERSION]: '0.1.0',
    })
  );

  const provider = new WebTracerProvider({
    resource: resource,
  });

  provider.addSpanProcessor(new BatchSpanProcessor(new ConsoleSpanExporter()));
  provider.addSpanProcessor(
    new BatchSpanProcessor(
      new OTLPTraceExporter({
        timeoutMillis: 15000,
        url: `${window.location.origin}/v1/webapi/traces`,
        concurrencyLimit: 10, // an optional limit on pending requests
      })
    )
  );

  provider.register({
    contextManager: new ZoneContextManager(),
    propagator: new CompositePropagator({
      propagators: [new B3Propagator(), new W3CTraceContextPropagator()],
    }),
  });
}
