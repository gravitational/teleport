import { createContext, PropsWithChildren, useContext } from 'react';

import { OktaIntegrationLevel } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';

type OktaIntegrationSetUpContextType = {
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  setPlugin: (plugin: Plugin<PluginOktaSpec, PluginStatusOkta>) => void;
  startFrom: OktaIntegrationLevel | undefined;
};

const OktaIntegrationSetUpContext =
  createContext<OktaIntegrationSetUpContextType>({
    plugin: undefined,
    setPlugin: () => {},
    startFrom: undefined,
  });

export const OktaIntegrationSetUpContextProvider = (
  props: PropsWithChildren<OktaIntegrationSetUpContextType>
) => (
  <OktaIntegrationSetUpContext.Provider value={props}>
    {props.children}
  </OktaIntegrationSetUpContext.Provider>
);

export const useOktaIntegrationSetUpContext = () =>
  useContext(OktaIntegrationSetUpContext);
