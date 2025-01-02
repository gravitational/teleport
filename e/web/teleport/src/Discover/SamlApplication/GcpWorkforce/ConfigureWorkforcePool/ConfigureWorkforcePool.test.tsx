import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import {
  idpMetadata,
  MockSamlApplicationContextProvider,
} from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import type { SamlIdpMetadataResponse } from 'e-teleport/services/idp/types';
import type { SamlIdpServiceProvider } from 'teleport/services/samlidp/types';

import {
  ConfigurePool,
  defaultSamlMetaForGcpWorkforce,
  isValidGcpOrgID,
  isValidGCPResourceName,
} from './ConfigureWorkforcePool';

const renderConfigureServiceProvider = (samlProviderProps: any) => {
  const samlApplicaitonContextProps = {
    runFetchMetadataValues: jest
      .fn()
      .mockImplementation(() => Promise<[SamlIdpMetadataResponse, Error]>),
    runUpsert: jest
      .fn()
      .mockImplementation(() => Promise<[SamlIdpServiceProvider, Error]>),
    upsertRequest: emptyUpsertRequest,
    guidedConfig: defaultSamlMetaForGcpWorkforce,
  };
  render(
    <MockSamlApplicationContextProvider
      samlProviderProps={{
        ...samlApplicaitonContextProps,
        ...samlProviderProps,
      }}
    >
      <ConfigurePool prevStep={() => null} nextStep={() => null} />
    </MockSamlApplicationContextProvider>
  );
};

test('toggle off disables guided config flow', async () => {
  const onToggle = jest.fn();
  renderConfigureServiceProvider({
    setGuidedToggle: onToggle,
    guidedToggle: true,
  });

  const guidedFlowEl = screen.getByText(
    'Guided configuration flow is enabled.'
  );
  expect(guidedFlowEl).toBeInTheDocument();
  expect(
    screen.getByText(
      'Generate an installation command to configure Workforce Identity Federation pool and pool provider'
    )
  ).toBeInTheDocument();

  fireEvent.click(guidedFlowEl);
  expect(onToggle).toHaveBeenCalledWith(false);
});

test('metadata UI visible on manual mode', async () => {
  renderConfigureServiceProvider({
    guidedToggle: false,
    fetchMetadataValuesAttempt: {
      status: 'success',
      data: idpMetadata,
      statusText: '',
    },
  });

  expect(
    screen.getByText('Guided configuration flow is disabled.')
  ).toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getByText('Teleport SAML IdP Metadata')).toBeInTheDocument();
  });
});

describe('isValidGcpOrgID', () => {
  test.each`
    orgId                  | valid
    ${'1234567890'}        | ${true}
    ${'123asbdac908'}      | ${false}
    ${'abc-123-qwe99'}     | ${false}
    ${'123458132-1234123'} | ${false}
    ${'1abc-123-qwe'}      | ${false}
  `('organization Id: $orgId', ({ orgId, valid }) => {
    const result = isValidGcpOrgID(orgId)();

    expect(result.valid).toEqual(valid);
  });
});

describe('isValidGCPResourceName', () => {
  test.each`
    names                                                                                 | valid
    ${'abc-asd-qwe'}                                                                      | ${true}
    ${'abc-123-qwe'}                                                                      | ${true}
    ${'abc-123-qwe99'}                                                                    | ${true}
    ${'abc-123-qweabc-123-qabc-123-qweabc-123-qabc-123-qweabc-123-qabc-123-qweabc-123-q'} | ${false}
    ${'abc-ABC-123-qwe'}                                                                  | ${false}
    ${'Aabc-123-qwe'}                                                                     | ${false}
    ${'abc-1_23-qwe'}                                                                     | ${false}
    ${'abc-123-qwe-'}                                                                     | ${false}
    ${'1abc-123-qwe'}                                                                     | ${false}
  `('names: $names', ({ names, valid }) => {
    const result = isValidGCPResourceName(names)();

    expect(result.valid).toEqual(valid);
  });
});
