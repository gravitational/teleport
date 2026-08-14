import { useMemo } from 'react';

import cfg from 'e-teleport/config';
import { CreateInferenceModel } from 'e-teleport/SessionRecordings/setup/create/CreateInferenceModel';
import { CreateInferencePolicy } from 'e-teleport/SessionRecordings/setup/create/CreateInferencePolicy';
import { CreateInferenceSecret } from 'e-teleport/SessionRecordings/setup/create/CreateInferenceSecret';
import { EditInferenceModel } from 'e-teleport/SessionRecordings/setup/edit/EditInferenceModel';
import { EditInferencePolicy } from 'e-teleport/SessionRecordings/setup/edit/EditInferencePolicy';
import { EditInferenceSecret } from 'e-teleport/SessionRecordings/setup/edit/EditInferenceSecret';
import {
  OverlayEntity,
  OverlayType,
  useSessionSummariesManagement,
} from 'e-teleport/SessionRecordings/setup/SessionSummariesManagement';

export function SessionSummariesOverlays() {
  const isCloud = cfg.oss.isCloud;

  const { overlays } = useSessionSummariesManagement();

  const items = useMemo(
    () =>
      overlays.map((overlay, index) => {
        if (overlay.type === OverlayType.Edit) {
          switch (overlay.entity) {
            case OverlayEntity.Model:
              return <EditInferenceModel key={index} name={overlay.name} />;

            case OverlayEntity.Policy:
              return (
                <EditInferencePolicy
                  key={index}
                  name={overlay.name}
                  isCloud={isCloud}
                />
              );

            case OverlayEntity.Secret:
              return <EditInferenceSecret key={index} name={overlay.name} />;
          }
        }

        switch (overlay.entity) {
          case OverlayEntity.Model:
            return <CreateInferenceModel key={index} />;

          case OverlayEntity.Policy:
            return <CreateInferencePolicy key={index} isCloud={isCloud} />;

          case OverlayEntity.Secret:
            return <CreateInferenceSecret key={index} />;
        }
      }),
    [overlays, isCloud]
  );

  return <>{items}</>;
}
