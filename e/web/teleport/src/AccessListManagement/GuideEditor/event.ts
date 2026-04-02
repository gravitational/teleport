import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';
import {
  AccessListEvent,
  AccessListIntegrateEvent,
  AccessListPresetEvent,
  AccessListStepStatusEvent,
} from 'teleport/services/userEvent/accessListEvents';

export type AccessListEmitEvent = {
  /**
   * If undefined, it will try to fallback to
   * the event name found in the current AccessListView
   */
  event?: AccessListEvent;
  stepStatus: AccessListStepStatusEvent;
  integrate?: AccessListIntegrateEvent;
  stepStatusError?: string;
  preferredTerraform?: boolean;
};

export function getAccessListPresetForEvent(preset: AccessListPreset) {
  switch (preset) {
    case 'long-term':
      return AccessListPresetEvent.LongTerm;
    case 'short-term':
      return AccessListPresetEvent.ShortTerm;
    case '':
      return AccessListPresetEvent.Unspecified;
    default:
      preset satisfies never;
      return AccessListPresetEvent.Unspecified;
  }
}
