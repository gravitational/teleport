import { ExternalAuditStorageDelete } from './IntegrationEnroll/DeleteDialogs/ExternalAuditStorageDelete';
import { PluginDelete } from './PluginDelete';

export default {
  title: 'TeleportE/Integrations/Delete',
};

export function ExternalAuditStorage() {
  return (
    <ExternalAuditStorageDelete
      onClose={() => {}}
      onDelete={async () => {}}
      opType="cluster"
    />
  );
}

export function Plugin() {
  return (
    <PluginDelete
      onClose={() => {}}
      onDelete={async () => {}}
      pluginKind="okta"
    />
  );
}
