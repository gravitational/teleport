import api from 'teleport/services/api';

import { ExternalAuditStorage } from 'teleport/services/integrations';
import { ConnectionDiagnostic } from 'teleport/services/agents';
import { makeConnectionDiagnostic } from 'teleport/services/agents/make';

import cfg from 'e-teleport/config';

export const externalAuditStorageService = {
  generateDraft(awsIntegrationName: string): Promise<ExternalAuditStorage> {
    return api
      .post(cfg.getExternalAuditStorageGenerateUrl(cfg.oss.proxyCluster), {
        integration_name: awsIntegrationName,
      })
      .then(makeExternalAuditStorage);
  },

  promoteDraft(): Promise<void> {
    return api.post(
      cfg.getExternalAuditStoragePromoteUrl(cfg.oss.proxyCluster)
    );
  },

  async getCluster(): Promise<ExternalAuditStorage | null> {
    try {
      const resp = await api.get(
        cfg.getExternalAuditStorageClusterUrl(cfg.oss.proxyCluster)
      );
      return makeExternalAuditStorage(resp);
    } catch (err: any) {
      // a 404 is expected when there is no active ExternalAuditStorage in the cluster,
      // so we return a null instead of throwing an exception
      if (err?.response?.status === 404) {
        return null;
      }
      throw err;
    }
  },

  async getDraft(): Promise<ExternalAuditStorage | null> {
    try {
      const resp = await api.get(
        cfg.getExternalAuditStorageDraftUrl(cfg.oss.proxyCluster)
      );
      return makeExternalAuditStorage(resp);
    } catch (err: any) {
      if (err?.response?.status === 404) {
        return null;
      }
    }
  },

  deleteCluster(): Promise<void> {
    return api.delete(
      cfg.getExternalAuditStorageClusterUrl(cfg.oss.proxyCluster)
    );
  },

  deleteDraft(): Promise<void> {
    return api.delete(
      cfg.getExternalAuditStorageDraftUrl(cfg.oss.proxyCluster)
    );
  },

  testConnection(): Promise<ConnectionDiagnostic> {
    return api
      .post(cfg.oss.getConnectionDiagnosticUrl(), {
        resource_kind: 'external_audit_storage',
        resource_name: 'draft',
      })
      .then(makeConnectionDiagnostic);
  },
};

export function makeExternalAuditStorage(json: any): ExternalAuditStorage {
  const {
    integration_name,
    policy_name,
    region,
    session_recordings_uri,
    athena_workgroup,
    glue_database,
    glue_table,
    audit_events_long_term_uri,
    athena_results_uri,
  } = json.spec;

  return {
    integrationName: integration_name,
    policyName: policy_name,
    region: region,
    sessionsRecordingsURI: session_recordings_uri,
    athenaWorkgroup: athena_workgroup,
    glueDatabase: glue_database,
    glueTable: glue_table,
    auditEventsLongTermURI: audit_events_long_term_uri,
    athenaResultsURI: athena_results_uri,
  };
}
