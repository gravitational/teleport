import { Labels } from 'teleport/services/resources';

export const TeleportOriginLabelKey = 'teleport.dev/origin';

export type LabelOption = {
  value: { labelKey: string; labelVals: string[] };
  label: string;
  hasError?: boolean;
};

export function hasLabels(labels: Labels) {
  if (!labels) {
    return false;
  }
  return !!Object.keys(labels).length;
}
