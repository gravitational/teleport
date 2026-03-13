/* oxlint-disable jest/no-conditional-expect */
import {
  act,
  fireEvent,
  render,
  screen,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import Validation, { useValidation } from 'shared/components/Validation';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  MockSamlApplicationContextProvider,
  mockSamlIdpServiceProvider,
  mockSamlMeta,
} from 'e-teleport/SamlApplication/fixtures';
import {
  emptyUpsertRequest,
  transformSamlSpecToCreateRequest,
} from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import type { SamlIdpMetadataResponse } from 'e-teleport/services/idp/types';
import { ContextProvider } from 'teleport/index';
import {
  SamlServiceProviderPreset,
  type AttributeMapping as AttributeMappingType,
  type SamlIdpServiceProvider,
} from 'teleport/services/samlidp/types';

import {
  AddMetadataGeneric,
  ConfigureServiceProvider,
  ErrMissingEntityIDOrACSURL,
} from './ConfigureServiceProvider';

const testED = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-23T03:28:35.58Z" entityID="https://example.com/saml/metadata">
<SPSSODescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" validUntil="2023-12-23T03:28:35.5797754Z" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" AuthnRequestsSigned="false" WantAssertionsSigned="true">
    <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</NameIDFormat>
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://example.com/saml/acs" index="1"></AssertionConsumerService>
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Artifact" Location="https://example.com/saml/acs" index="2"></AssertionConsumerService>
</SPSSODescriptor>
</EntityDescriptor>`;

const entityDescriptorLabelText =
  "Paste Service Provider entity descriptor's XML content. Please refer to your Service Provider's documentation for instructions on how to obtain the entity descriptor.";

const renderConfigureServiceProvider = (
  samlProviderProps: any,
  preset?: SamlServiceProviderPreset,
  isUpdateFlow?: boolean
) => {
  const ctx = createTeleportContextE();

  const samlApplicaitonContextProps = {
    runFetchMetadataValues: jest
      .fn()
      .mockImplementation(() => Promise<[SamlIdpMetadataResponse, Error]>),
    runUpsert: jest
      .fn()
      .mockImplementation(() => Promise<[SamlIdpServiceProvider, Error]>),
  };
  render(
    <ContextProvider ctx={ctx}>
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          ...samlApplicaitonContextProps,
          ...samlProviderProps,
        }}
      >
        <ConfigureServiceProvider
          header="samlAppHeader"
          subtitle="samlAppSubtitle"
          agentMeta={mockSamlMeta}
          updateAgentMeta={jest.fn()}
          prevStep={() => null}
          nextStep={() => null}
          SpMetadataConfigComponent={AddMetadataGeneric}
          preset={preset}
          isUpdateFlow={isUpdateFlow}
        />
      </MockSamlApplicationContextProvider>
    </ContextProvider>
  );
};

test('agentmeta is set on isUpdateFlow', async () => {
  const samlProviderProps = {
    setUpsertRequest: jest.fn(),
  };
  renderConfigureServiceProvider(
    samlProviderProps,
    SamlServiceProviderPreset.Unspecified,
    true /** isUpdateFlow */
  );

  expect(samlProviderProps.setUpsertRequest).toHaveBeenLastCalledWith(
    transformSamlSpecToCreateRequest(mockSamlMeta)
  );
});

test('preset is updated on props', async () => {
  const samlProviderProps = {
    setUpsertRequest: jest.fn(),
    isUpdateFlow: false,
  };

  renderConfigureServiceProvider(
    samlProviderProps,
    SamlServiceProviderPreset.GcpWorkforce,
    false /** isUpdateFlow */
  );

  expect(samlProviderProps.setUpsertRequest).toHaveBeenCalledWith(
    expect.objectContaining({ preset: SamlServiceProviderPreset.GcpWorkforce })
  );
});

test('upsert values sent on Finish button click', async () => {
  const runUpsert = jest
    .fn()
    .mockResolvedValueOnce([mockSamlIdpServiceProvider, null]);
  const upsertRequest = transformSamlSpecToCreateRequest({
    samlGeneric: mockSamlIdpServiceProvider,
  });
  renderConfigureServiceProvider(
    {
      upsertRequest: upsertRequest,
      runUpsert: runUpsert,
    },
    SamlServiceProviderPreset.Unspecified,
    true /** isUpdateFlow */
  );

  fireEvent.click(screen.getByRole('button', { name: /Finish/i }));
  await waitFor(() => {
    // TODO(sshah): figure why the following runUpsert is expecting trailing boolean value.
    expect(runUpsert).toHaveBeenCalledWith(upsertRequest, true);
  });
});

test('addMetadataGenric: disabled app name on isUpdateFlow props', async () => {
  render(
    <Validation>
      <AddMetadataGeneric
        spConfig={emptyUpsertRequest}
        setSPConfig={jest.fn()}
        isProcessing={false}
        isGuided={false}
        isUpdateFlow={true}
        setLabelsValidator={() => null}
      />
    </Validation>
  );

  expect(screen.getByPlaceholderText('app_saml')).toBeDisabled();

  // entity id and acs url fields must not be disabled on update.
  expect(
    screen.getByPlaceholderText('https://example.com/saml/metadata')
  ).toBeEnabled();
  expect(
    screen.getByPlaceholderText('https://example.com/saml/acs')
  ).toBeEnabled();
});

test('addMetadataGenric: disabled inputs on isProcessing props', async () => {
  render(
    <Validation>
      <AddMetadataGeneric
        spConfig={emptyUpsertRequest}
        setSPConfig={jest.fn()}
        isProcessing={true}
        isGuided={false}
        isUpdateFlow={false}
        setLabelsValidator={() => null}
      />
    </Validation>
  );

  expect(screen.getByPlaceholderText('app_saml')).toBeDisabled();
  expect(
    screen.getByPlaceholderText('https://example.com/saml/metadata')
  ).toBeDisabled();
  expect(
    screen.getByPlaceholderText('https://example.com/saml/acs')
  ).toBeDisabled();
  expect(screen.getByRole('button', { name: /add a label/i })).toBeDisabled();
});

test('addMetadataGenric: disabled inputs on guidedToggle and unspecified preset value', async () => {
  const samlProviderProps = {
    guidedToggle: true,
  };
  renderConfigureServiceProvider(
    samlProviderProps,
    SamlServiceProviderPreset.Unspecified
  );

  expect(screen.getByPlaceholderText('app_saml')).toBeEnabled();
  expect(
    screen.getByPlaceholderText('https://example.com/saml/metadata')
  ).toBeDisabled();
  expect(
    screen.getByPlaceholderText('https://example.com/saml/acs')
  ).toBeDisabled();

  expect(screen.getByRole('button', { name: /add a label/i })).toBeEnabled();
});

test('addMetadataGenric: disabled inputs on guidedToggle and GcpWorkforce preset value', async () => {
  const samlProviderProps = {
    guidedToggle: true,
  };
  renderConfigureServiceProvider(
    samlProviderProps,
    SamlServiceProviderPreset.GcpWorkforce
  );

  expect(screen.getByPlaceholderText('app_saml')).toBeDisabled();
  expect(
    screen.getByPlaceholderText('https://example.com/saml/metadata')
  ).toBeDisabled();
  expect(
    screen.getByPlaceholderText('https://example.com/saml/acs')
  ).toBeDisabled();

  expect(screen.getByRole('button', { name: /add a label/i })).toBeEnabled();
});

describe('addMetadataGeneric: onchange events', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  const tests: Array<{
    name: string;
    appName: string;
    entityID: string;
    acsURL: string;
    entityDescriptor: string;
    label?: { key: string; value: string };
  }> = [
    {
      name: 'with entityID and ACS URL',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
    },
    {
      name: 'with entity descriptor',
      appName: 'newApp',
      entityID: '',
      acsURL: '',
      entityDescriptor: testED,
    },
    {
      name: 'with attribute mapping',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
    },
    {
      name: 'all values',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: testED,
      label: { key: 'env', value: 'testing' },
    },
  ];

  test.each(tests)(
    '$name',
    async ({ appName, entityID, acsURL, entityDescriptor, label }) => {
      const user = userEvent.setup();
      const onChange = jest.fn();
      render(
        <Validation>
          <AddMetadataGeneric
            spConfig={emptyUpsertRequest}
            setSPConfig={onChange}
            isProcessing={false}
            isGuided={false}
            isUpdateFlow={false}
            setLabelsValidator={() => null}
          />
        </Validation>
      );

      fireEvent.change(screen.getByPlaceholderText('app_saml'), {
        target: { value: appName },
      });
      expect(onChange).toHaveBeenCalledWith(
        expect.objectContaining({ name: appName })
      );
      fireEvent.change(
        screen.getByPlaceholderText('https://example.com/saml/metadata'),
        { target: { value: entityID } }
      );
      expect(onChange).toHaveBeenCalledWith(
        expect.objectContaining({ entityID: entityID })
      );
      fireEvent.change(
        screen.getByPlaceholderText('https://example.com/saml/acs'),
        { target: { value: acsURL } }
      );
      expect(onChange).toHaveBeenCalledWith(
        expect.objectContaining({ acsURL: acsURL })
      );

      if (label) {
        await user.click(screen.getByRole('button', { name: /add a label/i }));
        fireEvent.change(screen.getByPlaceholderText('label key'), {
          target: { value: label.key },
        });
        fireEvent.change(screen.getByPlaceholderText('label value'), {
          target: { value: label.value },
        });
        expect(onChange).toHaveBeenCalledWith(
          expect.objectContaining({ labels: { [label.key]: label.value } })
        );
      }

      await user.click(screen.getByText('Add Entity Descriptor (Optional)'));
      const entityDescriptorEl = screen.getByText(entityDescriptorLabelText);
      await user.paste(entityDescriptor);
      await user.click(entityDescriptorEl);

      await waitFor(() => {
        expect(onChange).toHaveBeenCalledWith(
          expect.objectContaining({ entityDescriptor: entityDescriptor })
        );
      });
    }
  );
});

describe('addMetadataGeneric: onchange with errors', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });
  const tests: Array<{
    name: string;
    appName: string;
    entityID: string;
    acsURL: string;
    entityDescriptor: string;
    multipleError: boolean;
    error: string;
  }> = [
    {
      name: 'empty app name',
      appName: '',
      entityID: 'https://example.com/saml/metadata',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      error: 'Name is required',
      multipleError: false,
    },
    {
      name: 'entity ID and empty entity descriptor',
      appName: 'newApp',
      entityID: '',
      acsURL: 'https://example.com/saml/acs',
      entityDescriptor: '',
      error: ErrMissingEntityIDOrACSURL,
      multipleError: true,
    },
    {
      name: 'empty ACS URL and entity descriptor',
      appName: 'newApp',
      entityID: 'https://example.com/saml/metadata',
      acsURL: '',
      entityDescriptor: '',
      error: ErrMissingEntityIDOrACSURL,
      multipleError: true,
    },
  ];

  test.each(tests)(
    '$name',
    async ({
      appName,
      entityID,
      acsURL,
      entityDescriptor,
      error,
      multipleError,
    }) => {
      const user = userEvent.setup();
      const onChange = jest.fn();
      let validator = null;
      const Button = () => {
        validator = useValidation();
        return (
          <button data-testid="validate" onClick={() => validator.validate()} />
        );
      };
      render(
        <Validation>
          <AddMetadataGeneric
            spConfig={emptyUpsertRequest}
            setSPConfig={onChange}
            isProcessing={false}
            isGuided={false}
            isUpdateFlow={false}
            setLabelsValidator={() => null}
          />
          <Button />
        </Validation>
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

      await user.click(screen.getByText('Add Entity Descriptor (Optional)'));
      const entityDescriptorEl = screen.getByText(entityDescriptorLabelText);
      await user.paste(entityDescriptor);
      await user.click(entityDescriptorEl);

      await userEvent.click(screen.getByTestId('validate'));
      if (multipleError) {
        expect(screen.getAllByText(error)[0]).toBeInTheDocument();
      } else {
        expect(screen.getByText(error)).toBeInTheDocument();
      }
      act(() => validator.reset());
    }
  );
});

describe('attributeMapping: add another attribute mapping', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });
  const tests: Array<{
    name: string;
    attributeMapping: AttributeMappingType[];
    error: string;
  }> = [
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
  ];

  test.each(tests)('$name', async ({ attributeMapping, error }) => {
    const onChange = jest.fn();
    const samlProviderProps = {
      guidedToggle: false,
      setUpsertRequest: onChange,
      upsertRequest: {
        ...emptyUpsertRequest,
        // make one empty attribute mapping row appear by default.
        attributeMapping:
          emptyUpsertRequest.attributeMapping.concat(attributeMapping),
      },
    };

    renderConfigureServiceProvider(
      samlProviderProps,
      SamlServiceProviderPreset.Unspecified
    );

    fireEvent.click(
      screen.getByRole('button', { name: /Add Another Attribute Mapping/i })
    );

    if (error) {
      expect(screen.getByText(error)).toBeInTheDocument();
    } else {
      expect(onChange).toHaveBeenCalledWith(
        expect.objectContaining({
          attributeMapping:
            samlProviderProps.upsertRequest.attributeMapping.concat(
              emptyUpsertRequest.attributeMapping[0]
            ),
        })
      );
    }
  });
});
