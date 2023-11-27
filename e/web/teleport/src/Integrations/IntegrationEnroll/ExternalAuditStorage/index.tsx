import { ExternalAuditStorage } from './ExternalAuditStorage';
import { ExternalAuditStorageProvider } from './useExternalAuditStorage';

export default function ExternalAuditStorageIntegration() {
  return (
    <ExternalAuditStorageProvider>
      <ExternalAuditStorage />
    </ExternalAuditStorageProvider>
  );
}
