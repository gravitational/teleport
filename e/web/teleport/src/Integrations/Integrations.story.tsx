import { ExternalAuditStorageDelete } from './ExternalAuditStorageDelete';
import { PluginDelete } from './PluginDelete';

export default {
  title: 'TeleportE/Integrations',
};

export function DeleteExternalAuditStorage() {
  return (
    <ExternalAuditStorageDelete
      onClose={() => {}}
      onDelete={async () => {}}
      opType="cluster"
    />
  );
}

export function DeletePlugin() {
  return (
    <PluginDelete
      onClose={() => {}}
      onDelete={async () => {}}
      pluginKind="okta"
    />
  );
}
