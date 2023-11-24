import { ExternalAuditStorage } from './ExternalAuditStorage';
import { ExternalAuditStorageProvider } from './useExternalAuditStorage';

export default function ExternalCloudAudit() {
  return (
    <ExternalAuditStorageProvider>
      <ExternalAuditStorage />
    </ExternalAuditStorageProvider>
  );
}
