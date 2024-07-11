import { useState } from 'react';
import Validation from 'shared/components/Validation';

import { AttributeMapping } from './AttributeMapping';

export default {
  title: 'TeleportE/Discover/SAML Application/shared/AttributeMapping',
};

export const Default = () => {
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
        attempt={{ status: '' }}
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
        attempt={{ status: 'processing' }}
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
        attempt={{ status: '' }}
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
  attempt: { status: '' },
};
