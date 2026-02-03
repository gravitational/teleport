import { useMemo } from 'react';

import { CreateInferencePolicy } from 'e-teleport/Integrations/SessionSummaries/create/CreateInferencePolicy';
import { CreateInferenceSecret } from 'e-teleport/Integrations/SessionSummaries/create/CreateInferenceSecret';
import { EditInferencePolicy } from 'e-teleport/Integrations/SessionSummaries/edit/EditInferencePolicy';
import { EditInferenceSecret } from 'e-teleport/Integrations/SessionSummaries/edit/EditInferenceSecret';
import {
  OverlayEntity,
  OverlayType,
  useSessionSummariesManagement,
} from 'e-teleport/Integrations/SessionSummaries/SessionSummariesManagement';

export function SessionSummariesOverlays() {
  const { overlays } = useSessionSummariesManagement();

  const items = useMemo(
    () =>
      overlays.map((overlay, index) => {
        // missing overlays will be added in future PRs
        if (overlay.type === OverlayType.Edit) {
          switch (overlay.entity) {
            case OverlayEntity.Model:
              return null;

            case OverlayEntity.Policy:
              return <EditInferencePolicy key={index} name={overlay.name} />;

            case OverlayEntity.Secret:
              return <EditInferenceSecret key={index} name={overlay.name} />;
          }
        }

        switch (overlay.entity) {
          case OverlayEntity.Model:
            return null;

          case OverlayEntity.Policy:
            return <CreateInferencePolicy key={index} />;

          case OverlayEntity.Secret:
            return <CreateInferenceSecret key={index} />;
        }
      }),
    [overlays]
  );

  return <>{items}</>;
}
