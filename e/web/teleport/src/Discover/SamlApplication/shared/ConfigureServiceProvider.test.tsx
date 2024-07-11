import React from 'react';
import {
  fireEvent,
  render,
  screen,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import { AgentMeta } from 'teleport/Discover/useDiscover';

import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import {
  ConfigureServiceProvider,
  AddMetadataGeneric,
  ErrMissingEntityIDOrACSURL,
  genEntityIDAndAcsUrlForGcpWorkforce,
} from './ConfigureServiceProvider';

import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

import type { AttributeMapping } from 'teleport/services/samlidp/types';

const testED = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-23T03:28:35.58Z" entityID="https://example.com/saml/metadata">
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-23T03:28:35.5797754Z" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
    <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</NameIDFormat>
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://example.com/saml/acs" index="1"></AssertionConsumerService>
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact" Location="https://example.com/saml/acs" index="2"></AssertionConsumerService>
</SPSSODescriptor>
</EntityDescriptor>`;

const entityDescriptorLabelText =
  "Paste Service Provider entity descriptor's XML content. Please refer to your Service Provider's documentation for instructions on how to obtain the entity descriptor.";

describe('configure SAML service provider', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  const tests: Array<{
    name: string;
    appName: string;
    entityID: string;
    acsURL: string;
    entityDescriptor: string;
    attributeMapping: AttributeMapping[];
  }> = [
    {
      name: 'with entityID and ACS URL',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      attributeMapping: [],
    },
    {
      name: 'with entity descriptor',
      appName: 'newApp',
      entityID: '',
      acsURL: '',
      entityDescriptor: testED,
      attributeMapping: [],
    },
    {
      name: 'with attribute mapping',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      attributeMapping: [
        {
          name: 'firstname',
          name_format: 'unspecified',
          value: 'user.spec.traits.firstname',
        },
      ],
    },
    {
      name: 'all values',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: testED,
      attributeMapping: [
        {
          name: 'groups',
          name_format: 'unspecified',
          value: 'user.spec.traits.groups',
        },
      ],
    },
  ];

  test.each(tests)(
    '$name',
    async ({
      appName,
      entityID,
      acsURL,
      entityDescriptor,
      attributeMapping,
    }) => {
      const user = userEvent.setup();
      const onSubmit = jest.fn();
      render(
        <ConfigureServiceProvider
          header="samlAppHeader"
          subtitle="samlAppSubtitle"
          attempt={{ status: '' }}
          agentMeta={{} as AgentMeta}
          updateAgentMeta={jest.fn()}
          upsertSP={onSubmit}
          prevStep={() => null}
          nextStep={() => null}
          SpMetadataConfigComponent={AddMetadataGeneric}
          isUpdateFlow={false}
        />
      );

      expect(screen.getByText('samlAppHeader')).toBeInTheDocument();
      expect(screen.getByText('samlAppSubtitle')).toBeInTheDocument();

      await user.type(screen.getByPlaceholderText('app_saml'), appName);
      fireEvent.change(
        screen.getByPlaceholderText('https://example.com/saml/metadata'),
        { target: { value: entityID } }
      );
      fireEvent.change(
        screen.getByPlaceholderText('https://example.com/saml/acs'),
        { target: { value: acsURL } }
      );

      await user.click(screen.getByText('Add Entity Descriptor (optional)'));
      const entityDescriptorEl = screen.getByText(entityDescriptorLabelText);
      await user.paste(entityDescriptor);
      await user.click(entityDescriptorEl);

      for (const attribute of attributeMapping) {
        await user.type(
          screen.getByPlaceholderText('attribute_name'),
          attribute.name
        );
        const attrValEl = screen.getByLabelText('attribute value');
        await user.type(attrValEl, attribute.value);
        await user.type(attrValEl, '{enter}');
      }

      await user.click(screen.getByRole('button', { name: /Finish/i }));

      await waitFor(() => {
        expect(onSubmit).toHaveBeenLastCalledWith({
          name: appName,
          entityID: entityID,
          acsURL: acsURL,
          entityDescriptor: entityDescriptor,
          attributeMapping: attributeMapping,
        });
      });
    }
  );
});

describe('configure SAML service provider with errors', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });
  const tests: Array<{
    name: string;
    appName: string;
    entityID: string;
    acsURL: string;
    entityDescriptor: string;
    attributeMapping: AttributeMapping[];
    error: string;
  }> = [
    {
      name: 'empty app name',
      appName: '',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      attributeMapping: [],
      error: 'Name is required',
    },
    {
      name: 'empty entity ID and entity descriptor',
      appName: 'newApp',
      entityID: '',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      attributeMapping: [],
      error: ErrMissingEntityIDOrACSURL,
    },
    {
      name: 'empty ACS URL and entity descriptor',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: '',
      entityDescriptor: '',
      attributeMapping: [],
      error: ErrMissingEntityIDOrACSURL,
    },
    {
      name: 'empty attribute name',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      attributeMapping: [
        { name: '', name_format: '', value: 'user.spec.traits.groups' },
      ],
      error: 'Attribute name cannot be empty',
    },
    {
      name: 'empty attribute value',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      attributeMapping: [{ name: 'firstname', name_format: '', value: '' }],
      error: 'Attribute value cannot be empty',
    },
  ];

  test.each(tests)(
    '$name',
    async ({
      appName,
      entityID,
      acsURL,
      entityDescriptor,
      attributeMapping,
      error,
    }) => {
      const user = userEvent.setup();
      const onSubmit = jest.fn();
      render(
        <ConfigureServiceProvider
          header="samlAppHeader"
          subtitle="samlAppSubtitle"
          attempt={{ status: '' }}
          agentMeta={{} as AgentMeta}
          updateAgentMeta={jest.fn()}
          upsertSP={onSubmit}
          prevStep={() => null}
          nextStep={() => null}
          SpMetadataConfigComponent={AddMetadataGeneric}
          isUpdateFlow={false}
        />
      );

      fireEvent.change(screen.getByPlaceholderText('app_saml'), {
        target: { value: appName },
      });
      fireEvent.change(
        screen.getByPlaceholderText('https://example.com/saml/metadata'),
        { target: { value: entityID } }
      );
      fireEvent.change(
        screen.getByPlaceholderText('https://example.com/saml/acs'),
        { target: { value: acsURL } }
      );

      await user.click(screen.getByText('Add Entity Descriptor (optional)'));
      const entityDescriptorEl = screen.getByText(entityDescriptorLabelText);
      await user.paste(entityDescriptor);
      await user.click(entityDescriptorEl);

      for (const attribute of attributeMapping) {
        fireEvent.change(screen.getByPlaceholderText('attribute_name'), {
          target: { value: attribute.name },
        });
        // checking if value is empty here otherwise, with the change and '{enter}'
        // event, test will always select the first attribute value from option list.
        if (attribute.value.length > 0) {
          const attrValEl = screen.getByLabelText('attribute value');
          fireEvent.change(attrValEl, {
            target: { value: attribute.value },
          });
          await user.type(attrValEl, '{enter}');
        }
      }

      await user.click(screen.getByRole('button', { name: /Finish/i }));
      expect(screen.getByText(error)).toBeInTheDocument();
    }
  );
});

describe('add another attribute mapping with errors', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });
  const tests: Array<{
    name: string;
    attributeMapping: AttributeMapping[];
    error: string;
  }> = [
    {
      name: 'add new attribute fields if both name and value is set',
      attributeMapping: [
        {
          name: 'firstname',
          name_format: 'unspecified',
          value: 'user.spec.traits.firstname',
        },
      ],
      error: '',
    },
    {
      name: 'do not add new attribute fields if both name and value is missing',
      attributeMapping: [],
      error: 'Attribute name cannot be empty',
    },
    {
      name: 'do not add new attribute fields if name is missing',
      attributeMapping: [
        { name: '', name_format: '', value: 'user.spec.traits.firstname' },
      ],
      error: 'Attribute name cannot be empty',
    },
    {
      name: 'do not add new attribute fields if value is missing',
      attributeMapping: [{ name: 'firstname', name_format: '', value: '' }],
      error: 'Attribute value cannot be empty',
    },
  ];

  test.each(tests)('$name', async ({ attributeMapping, error }) => {
    const user = userEvent.setup();
    const onSubmit = jest.fn();
    render(
      <ConfigureServiceProvider
        header="samlAppHeader"
        subtitle="samlAppSubtitle"
        attempt={{ status: '' }}
        agentMeta={{} as AgentMeta}
        updateAgentMeta={jest.fn()}
        upsertSP={onSubmit}
        prevStep={() => null}
        nextStep={() => null}
        SpMetadataConfigComponent={AddMetadataGeneric}
        isUpdateFlow={false}
      />
    );

    for (const attribute of attributeMapping) {
      fireEvent.change(screen.getByPlaceholderText('attribute_name'), {
        target: { value: attribute.name },
      });
      // checking if value is empty here otherwise, with the change and '{enter}'
      // event, test will always select the first attribute value from option list.
      if (attribute.value) {
        const attrValEl = screen.getByLabelText('attribute value');
        fireEvent.change(attrValEl, {
          target: { value: attribute.value },
        });
        await user.type(attrValEl, '{enter}');
      }
    }

    await userEvent.click(screen.getByText('Add Another Attribute Mapping'));

    if (error) {
      // eslint-disable-next-line jest/no-conditional-expect
      expect(screen.getByText(error)).toBeInTheDocument();
    } else {
      const attrNameEl = screen.getAllByPlaceholderText('attribute_name');
      // eslint-disable-next-line jest/no-conditional-expect
      expect(attrNameEl).toHaveLength(2);
    }
  });
});

describe('metadada and attribute mapping renders based on resourceSpec preset', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  const poolName = 'test-pool';
  const poolProviderName = 'test-pool-provider';
  const renderConfigureServiceProvider = () => {
    render(
      <ConfigureServiceProvider
        header="samlAppHeader"
        subtitle="samlAppSubtitle"
        attempt={{ status: '' }}
        agentMeta={{
          samlGcpWorkforce: {
            isAutoConfig: true,
            orgId: '',
            poolName: poolName,
            poolProviderName: poolProviderName,
          },
        }}
        updateAgentMeta={jest.fn()}
        upsertSP={jest.fn()}
        prevStep={() => null}
        nextStep={() => null}
        SpMetadataConfigComponent={AddMetadataGeneric}
        resourceSpec={
          {
            samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
          } as ResourceSpec
        }
        isUpdateFlow={false}
      />
    );
  };

  test('disable input based on resourceSpec preset and SamlMeta isAutoConfig', async () => {
    renderConfigureServiceProvider();

    expect(screen.getByPlaceholderText('app_saml')).toBeDisabled();
    expect(
      screen.getByPlaceholderText('https://example.com/saml/metadata')
    ).toBeDisabled();
    expect(
      screen.getByPlaceholderText('https://example.com/saml/acs')
    ).toBeDisabled();
  });

  test('input fields are prepolutated with values', async () => {
    renderConfigureServiceProvider();

    const appNameEl = screen.getByPlaceholderText('app_saml');
    expect(appNameEl).toHaveDisplayValue('test-pool-provider');

    const entityIdAndAcsUrl = genEntityIDAndAcsUrlForGcpWorkforce(
      poolName,
      poolProviderName
    );
    const entityIdEl = screen.getByPlaceholderText(
      'https://example.com/saml/metadata'
    );
    expect(entityIdEl).toHaveDisplayValue(entityIdAndAcsUrl.entityId);
    const acsUrlEl = screen.getByPlaceholderText(
      'https://example.com/saml/acs'
    );
    expect(acsUrlEl).toHaveDisplayValue(entityIdAndAcsUrl.acsUrl);
  });
});
