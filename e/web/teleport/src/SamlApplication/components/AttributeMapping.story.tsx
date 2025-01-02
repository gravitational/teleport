import { useState } from 'react';

import Validation from 'shared/components/Validation';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { AttributeMapping } from './AttributeMapping';

export default {
  title: 'TeleportE/SamlApplication/components/AttributeMapping',
};

export const Default = () => {
  const [spConfig, setSPConfig] = useState<CreateSamlIdpServiceProviderRequest>(
    {
      name: '',
      entityID: '',
      acsURL: '',
      entityDescriptor: '',
      attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
    }
  );
  function addAttrMap() {
    setSPConfig({
      ...spConfig,
      attributeMapping: [
        ...spConfig.attributeMapping,
        { name: '', name_format: 'unspecified', value: '' },
      ],
    });
  }

  return (
    <Validation>
      <AttributeMapping
        {...props}
        spConfig={spConfig}
        setSPConfig={setSPConfig}
        addAttrMap={addAttrMap}
        disabled={false}
      />
    </Validation>
  );
};

export const Disabled = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: '',
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  });
  function addAttrMap() {
    setSPConfig({
      ...spConfig,
      attributeMapping: [
        ...spConfig.attributeMapping,
        { name: '', name_format: 'unspecified', value: '' },
      ],
    });
  }

  return (
    <Validation>
      <AttributeMapping
        {...props}
        spConfig={spConfig}
        setSPConfig={setSPConfig}
        addAttrMap={addAttrMap}
        disabled={true}
      />
    </Validation>
  );
};

export const ErrorField = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: '',
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  });
  const [attrMapErr, setAttrMapErr] = useState({
    emptyName: true,
    emptyValue: true,
  });
  function addAttrMap() {
    setSPConfig({
      ...spConfig,
      attributeMapping: [
        ...spConfig.attributeMapping,
        { name: '', name_format: 'unspecified', value: '' },
      ],
    });
  }

  return (
    <Validation>
      <AttributeMapping
        {...props}
        spConfig={spConfig}
        setSPConfig={setSPConfig}
        addAttrMap={addAttrMap}
        attrMapErr={attrMapErr}
        setAttrMapErr={setAttrMapErr}
        disabled={false}
      />
    </Validation>
  );
};

const props = {
  spConfig: {
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: '',
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  },
  setSPConfig: () => null,
  attrMapErr: {
    emptyName: false,
    emptyValue: false,
  },
  setAttrMapErr: () => null,
  addAttrMap: () => null,
  preset: SamlServiceProviderPreset.Unspecified,
  isGuided: false,
};
