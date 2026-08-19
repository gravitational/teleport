import React, { createContext, useCallback, useContext, useState } from 'react';

import { Attempt, useAsync } from 'shared/hooks/useAsync';

import type {
  CreateSamlIdpServiceProviderRequest,
  SamlIdpMetadataResponse,
} from 'e-teleport/services/idp/types';
import useTeleportE from 'e-teleport/useTeleportE';
import type { SamlMeta } from 'teleport/Discover/useDiscover';
import {
  SamlServiceProviderPreset,
  type AttributeMapping,
  type SamlIdpServiceProvider,
} from 'teleport/services/samlidp/types';

/**
 * SamlApplication defines type for Saml application create action.
 */
export interface SamlApplication {
  /**
   * runFetchMetadataValues fetches SAML IdP metadata values. Caller should use
   * fetchMetadataValuesAttempt to retreive data and status of runFetchMetadataValues.
   */
  runFetchMetadataValues: () => Promise<[SamlIdpMetadataResponse, Error]>;
  /**
   * fetchMetadataValuesAttempt is an async state runFetchMetadataValues.
   */
  fetchMetadataValuesAttempt: Attempt<SamlIdpMetadataResponse>;
  /**
   * upsertRequest holds service provider create/update request params.
   */
  upsertRequest: CreateSamlIdpServiceProviderRequest;
  /**
   * setUpsertRequest sets values for upsertRequest.
   */
  setUpsertRequest: (param: CreateSamlIdpServiceProviderRequest) => void;
  /**
   * runUpsert creates or updates service provider.
   * @param req holds value of upsertRequest.
   * @param isUpdateFlow is used to determine whether to create or update service provider.
   */
  runUpsert: (
    req: CreateSamlIdpServiceProviderRequest,
    isUpdateFlow: boolean
  ) => Promise<[SamlIdpServiceProvider, Error]>;
  /**
   * upsertAttempt is an async state of runUpsert.
   */
  upsertAttempt: Attempt<SamlIdpServiceProvider>;
  /**
   * guidedToggle is the guided configuration toggle state. Truthy value means guided configuration is enabled.
   */
  guidedToggle: boolean;
  /**
   * setIsGuidedToggle is used to set guided configuration toggle.
   */
  setGuidedToggle: (boolean) => void;
  /**
   * guidedConfig is used to store input state for guided configuration. These values are only
   * used to either generate values for upsertRequest or generate Teleport IdP installation script
   * that can be run in the 3rd party service.
   */
  guidedConfig: SamlMeta;
  /**
   * setGuidedConfig sets configuration parameters for guidedConfig.
   */
  setGuidedConfig: React.Dispatch<React.SetStateAction<SamlMeta>>;
}

const SamlApplicationContext = createContext<SamlApplication>(null);

export function useSamlApplication() {
  const context = useContext(SamlApplicationContext);
  if (!context) {
    throw new Error(
      'SamlApplicationContext must be used within a SamlApplicationProvider'
    );
  }
  return context;
}

type SamlApplicationProviderProps = {
  // mockCtx used for testing purposes.
  mockCtx?: SamlApplication;
};

/**
 * SamlApplicationProvider provides context for SamlApplication create action.
 * For the edit and delete action which is based on unified resource view,
 * use SamlAppActionProvider (useSamlAppActionsE.tsx).
 */
export function SamlApplicationProvider({
  mockCtx,
  children,
}: React.PropsWithChildren<SamlApplicationProviderProps>) {
  const { idpService } = useTeleportE();
  const [fetchMetadataValuesAttempt, runFetchMetadataValues] = useAsync(
    useCallback(() => idpService.getIdPMetadataValues(), [idpService])
  );

  const [guidedToggle, setGuidedToggle] = useState(null);
  const [guidedConfig, setGuidedConfig] = useState<SamlMeta>(null);

  const [upsertRequest, setUpsertRequest] =
    useState<CreateSamlIdpServiceProviderRequest>(emptyUpsertRequest);
  const [upsertAttempt, runUpsert] = useAsync(
    useCallback(
      (req: CreateSamlIdpServiceProviderRequest, isUpdateFlow: boolean) =>
        idpService.upsertRequest(req, isUpdateFlow),
      [idpService]
    )
  );

  const value: SamlApplication = {
    runFetchMetadataValues,
    fetchMetadataValuesAttempt,
    upsertRequest,
    setUpsertRequest,
    runUpsert,
    upsertAttempt,
    setGuidedToggle,
    guidedToggle,
    guidedConfig,
    setGuidedConfig,
  };

  return (
    <SamlApplicationContext.Provider value={mockCtx || value}>
      {children}
    </SamlApplicationContext.Provider>
  );
}

export function genEntityIDAndAcsUrlForGcpWorkforce(
  poolName: string,
  poolProviderName: string
) {
  const entityId = `https://iam.googleapis.com/locations/global/workforcePools/${poolName}/providers/${poolProviderName}`;
  const acsUrl = `https://auth.cloud.google/signin-callback/locations/global/workforcePools/${poolName}/providers/${poolProviderName}`;
  return { entityId, acsUrl };
}

export function transformSamlSpecToCreateRequest(
  samlMeta: SamlMeta
): CreateSamlIdpServiceProviderRequest {
  return {
    name: samlMeta.samlGeneric?.metadata?.name || '',
    labels: samlMeta.samlGeneric?.metadata?.labels || {},
    entityID: samlMeta.samlGeneric?.spec?.entity_id || '',
    acsURL: samlMeta.samlGeneric?.spec?.acs_url || '',
    entityDescriptor: samlMeta.samlGeneric?.spec?.entity_descriptor || '',
    attributeMapping: samlMeta.samlGeneric?.spec?.attribute_mapping || [
      { name: '', name_format: 'unspecified', value: '' },
    ],
    preset:
      samlMeta.samlGeneric?.spec?.preset ||
      SamlServiceProviderPreset.Unspecified,
  };
}

export const emptyUpsertRequest: CreateSamlIdpServiceProviderRequest = {
  name: '',
  labels: {},
  entityID: '',
  acsURL: '',
  entityDescriptor: '',
  attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  preset: SamlServiceProviderPreset.Unspecified,
};

export function checkDefaultAttributePerPreset(
  preset: SamlServiceProviderPreset,
  attributes: AttributeMapping[]
) {
  let exists: boolean = false;

  if (preset === SamlServiceProviderPreset.GcpWorkforce) {
    for (let a in attributes) {
      if (attributes[a].name === 'roles') {
        exists = true;
      }
    }
  }

  if (preset === SamlServiceProviderPreset.GcpWorkforce) {
    for (let a in attributes) {
      if (
        attributes[a].name ===
        'http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress'
      ) {
        exists = true;
      }
    }
  }

  return exists;
}
