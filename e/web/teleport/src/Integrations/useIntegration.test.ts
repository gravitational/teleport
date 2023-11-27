import { getWarningMessage } from './useIntegrations';

import type {
  ExternalAuditStorage,
  IntegrationListResponse,
  Plugin,
} from 'teleport/services/integrations';

describe('getErrorsMessage', () => {
  const makeFulfilled = <T>(): PromiseSettledResult<T> => ({
    status: 'fulfilled',
    value: {} as T,
  });
  const makeRejected = <T>(err: string): PromiseSettledResult<T> => ({
    reason: err,
    status: 'rejected',
  });

  it('returns an empty string if there are no errors', () => {
    let result = getWarningMessage(
      makeFulfilled<Plugin[]>(),
      makeFulfilled<IntegrationListResponse>(),
      makeFulfilled<ExternalAuditStorage>()
    );
    expect(result).toBe('');
  });

  it('returns correctly there are three errors', () => {
    let result = getWarningMessage(
      makeRejected<Plugin[]>('some error with plugins'),
      makeRejected<IntegrationListResponse>('integration err'),
      makeRejected<ExternalAuditStorage>('external audit storage err')
    );
    expect(result).toBe(
      'An error has occurred. PLUGINS: some error with plugins, INTEGRATIONS: integration err, EXTERNAL AUDIT: external audit storage err'
    );
  });

  it('returns correctly when there is one error', () => {
    let result = getWarningMessage(
      makeRejected<Plugin[]>('some error with plugins'),
      makeFulfilled<IntegrationListResponse>(),
      makeFulfilled<ExternalAuditStorage>()
    );
    expect(result).toBe(
      'Failed to fetch plugin integrations (try refreshing browser or check your "plugin" access): some error with plugins'
    );

    result = getWarningMessage(
      makeFulfilled<Plugin[]>(),
      makeRejected<IntegrationListResponse>(
        'no permission to fetch integrations'
      ),
      makeFulfilled<ExternalAuditStorage>()
    );
    expect(result).toBe(
      'Failed to fetch rest of integrations (try refreshing browser or check your "integration" access): no permission to fetch integrations'
    );

    result = getWarningMessage(
      makeFulfilled<Plugin[]>(),
      makeFulfilled<IntegrationListResponse>(),
      makeRejected<ExternalAuditStorage>('no permission to fetch cluster audit')
    );
    expect(result).toBe(
      'Failed to fetch external audit integration (try refreshing browser or check your "external_audit_storage" access): no permission to fetch cluster audit'
    );
  });

  it('returns correctly when there is are two errors', () => {
    let result = getWarningMessage(
      makeRejected<Plugin[]>('some error with plugins'),
      makeRejected<IntegrationListResponse>('some error with integrations'),
      makeFulfilled<ExternalAuditStorage>()
    );
    expect(result).toBe(
      'Failed to fetch plugin integrations and rest of integrations (try refreshing browser or check your "plugin" and "integration" access): some error with plugins and some error with integrations'
    );

    result = getWarningMessage(
      makeRejected<Plugin[]>('no permission to fetch plugins'),
      makeFulfilled<IntegrationListResponse>(),
      makeRejected<ExternalAuditStorage>(
        'no permission to fetch external audit'
      )
    );
    expect(result).toBe(
      'Failed to fetch external audit integration and plugin integrations (try refreshing browser or check your "external_audit_storage" and "plugin" access): no permission to fetch external audit and no permission to fetch plugins'
    );

    result = getWarningMessage(
      makeFulfilled<Plugin[]>(),
      makeRejected<IntegrationListResponse>('some error with integrations'),
      makeRejected<ExternalAuditStorage>(
        'no permission to fetch external audit'
      )
    );
    expect(result).toBe(
      'Failed to fetch external audit integration and rest of integrations (try refreshing browser or check your "external_audit_storage" and "integration" access): no permission to fetch external audit and some error with integrations'
    );
  });
});
