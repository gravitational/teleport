import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';
import { Support } from 'teleport/Support';

export default function Container() {
  return (
    <Support>
      <SupportE />
    </Support>
  );
}

export const SupportE = () => {
  return <ExternalAuditStorageCta />;
};
