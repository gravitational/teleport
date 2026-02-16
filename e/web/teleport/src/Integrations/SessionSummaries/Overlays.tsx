import { useMemo } from 'react';

import { CreateInferenceModel } from 'e-teleport/Integrations/SessionSummaries/create/CreateInferenceModel';
import { CreateInferencePolicy } from 'e-teleport/Integrations/SessionSummaries/create/CreateInferencePolicy';
import { CreateInferenceSecret } from 'e-teleport/Integrations/SessionSummaries/create/CreateInferenceSecret';
import { EditInferenceModel } from 'e-teleport/Integrations/SessionSummaries/edit/EditInferenceModel';
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
        if (overlay.type === OverlayType.Edit) {
          switch (overlay.entity) {
            case OverlayEntity.Model:
              return <EditInferenceModel key={index} name={overlay.name} />;

            case OverlayEntity.Policy:
              return <EditInferencePolicy key={index} name={overlay.name} />;

            case OverlayEntity.Secret:
              return <EditInferenceSecret key={index} name={overlay.name} />;
          }
        }

        switch (overlay.entity) {
          case OverlayEntity.Model:
            return <CreateInferenceModel key={index} />;

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
