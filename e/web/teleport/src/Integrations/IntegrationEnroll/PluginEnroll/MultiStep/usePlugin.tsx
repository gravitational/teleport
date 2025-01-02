import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { useLocation } from 'react-router';

import { getSuccessPrimaryButtonState } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/redirect';
import {
  pluginTypeToIntegrationEnrollKind,
  type CloudHostablePlugin,
  type View,
} from 'e-teleport/services/plugins';
import {
  addIndexToViews,
  findViewAtIndex,
} from 'teleport/components/Wizard/flow';
import { Plugin } from 'teleport/services/integrations';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';

export interface PluginContextState<T = any> {
  currentStep: number;
  selectedPlugin: CloudHostablePlugin;
  installedPlugin: Plugin<T>;
  setInstalledPlugin: (p: Plugin<T>) => void;
  nextStep: () => void;
  prevStep: () => void;
  indexedViews: View[];
  eventId: string;
  formData: FormData;

  setFormData(f: FormData): void;

  successPrimaryButtonUrl: string | null;
  successPrimaryButtonText: string | null;
}

const pluginContext = createContext<PluginContextState>(null);

export function PluginProvider<T>({
  selectedPlugin,
  children,
}: React.PropsWithChildren<{
  selectedPlugin: CloudHostablePlugin;
}>) {
  const [currentStep, setCurrentStep] = useState(0);
  const [installedPlugin, setInstalledPlugin] = useState<Plugin<T>>();
  const [formData, setFormData] = useState<FormData>(new FormData());
  const [eventId] = useState(() => crypto.randomUUID());

  // indexedViews contains views (including nested views)
  // of the selected plugin where
  // each view has been assigned an index value.
  // This is used to later look up views by target index.
  const [indexedViews] = useState<View[] | null>(() =>
    selectedPlugin.views ? addIndexToViews(selectedPlugin.views()) : null
  );

  const location = useLocation();

  const { successPrimaryButtonText, successPrimaryButtonUrl } = useMemo(
    () => getSuccessPrimaryButtonState(location.search),
    [location.search]
  );

  useEffect(() => {
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Started,
      eventData: {
        id: eventId,
        kind: pluginTypeToIntegrationEnrollKind(selectedPlugin.type),
      },
    });
    // Only send a Start event once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function nextStep() {
    const numToIncrement = 1;

    const nextView = findViewAtIndex(
      indexedViews,
      currentStep + numToIncrement
    );
    if (nextView) {
      setCurrentStep(currentStep + numToIncrement);
    }
  }

  function prevStep() {
    if (currentStep === 0) {
      // TODO(lisa):
      // Create a integration abort event
      // since we are starting over with plugin selection?
      // eg: emitEvent({ stepStatus: DiscoverEventStatus.Aborted });
      return;
    }

    const updatedCurrentStep = currentStep - 1;
    const nextView = findViewAtIndex(indexedViews, updatedCurrentStep);
    if (nextView) {
      setCurrentStep(updatedCurrentStep);
    }
  }

  const value: PluginContextState = {
    installedPlugin,
    setInstalledPlugin,
    formData,
    setFormData,
    currentStep,
    nextStep,
    prevStep,
    indexedViews,
    selectedPlugin,
    eventId,
    successPrimaryButtonUrl,
    successPrimaryButtonText,
  };

  return (
    <pluginContext.Provider value={value}>{children}</pluginContext.Provider>
  );
}

export function usePlugin<T>(): PluginContextState<T> {
  return useContext(pluginContext);
}
