import { Support } from 'teleport/Support';

import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';

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
