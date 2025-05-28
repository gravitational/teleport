import {
  createContext,
  PropsWithChildren,
  useCallback,
  useContext,
  useMemo,
} from 'react';

import {
  OktaIntegrationStepType,
  type OktaIntegrationLevelStep,
  type OktaIntegrationStepWithEnabled,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';

interface OktaIntegrationSetUpContextProviderProps {
  completedStepTypes: OktaIntegrationStepType[];
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  steps: OktaIntegrationStepWithEnabled[];
  startFrom: OktaIntegrationStepType | undefined;
}

interface OktaIntegrationSetUpContextValue
  extends OktaIntegrationSetUpContextProviderProps {
  getNextStep: (
    currentStepType: OktaIntegrationStepType | undefined
  ) => OktaIntegrationLevelStep | undefined;
  getPreviousStep: (
    currentStepType: OktaIntegrationStepType
  ) => OktaIntegrationLevelStep | undefined;
}

const OktaIntegrationSetUpContext =
  createContext<OktaIntegrationSetUpContextValue>({
    completedStepTypes: [],
    plugin: undefined,
    steps: [],
    startFrom: undefined,
    getNextStep: () => undefined,
    getPreviousStep: () => undefined,
  });

export const OktaIntegrationSetUpContextProvider = (
  props: PropsWithChildren<OktaIntegrationSetUpContextProviderProps>
) => {
  const { steps } = props;

  const enabledSteps = useMemo(
    () => steps.filter(step => step.enabled),
    [steps]
  );

  const getNextStep = useCallback(
    (
      currentStepType: OktaIntegrationStepType | undefined
    ): OktaIntegrationLevelStep | undefined => {
      if (!currentStepType) {
        return enabledSteps.find(
          config => config.type === OktaIntegrationStepType.Sso
        );
      }

      const index = enabledSteps.findIndex(
        level => level.type === currentStepType
      );

      if (index === -1 || index === enabledSteps.length - 1) {
        return undefined;
      }

      return enabledSteps[index + 1];
    },
    [enabledSteps]
  );

  const getPreviousStep = useCallback(
    (
      currentStepType: OktaIntegrationStepType
    ): OktaIntegrationLevelStep | undefined => {
      const index = enabledSteps.findIndex(
        level => level.type === currentStepType
      );

      if (index === -1 || index === 0) {
        return undefined;
      }

      return enabledSteps[index - 1];
    },
    [enabledSteps]
  );

  return (
    <OktaIntegrationSetUpContext.Provider
      value={{
        ...props,
        getNextStep,
        getPreviousStep,
      }}
    >
      {props.children}
    </OktaIntegrationSetUpContext.Provider>
  );
};

export const useOktaIntegrationSetUpContext = () =>
  useContext(OktaIntegrationSetUpContext);
