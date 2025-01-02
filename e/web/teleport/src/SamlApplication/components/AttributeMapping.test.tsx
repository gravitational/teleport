import { fireEvent, render, screen, userEvent } from 'design/utils/testing';
import Validation, { useValidation } from 'shared/components/Validation';

import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { AttributeMapping } from './AttributeMapping';

test('attribute mapping onchange', async () => {
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
      <AttributeMapping
        spConfig={emptyUpsertRequest}
        setSPConfig={onChange}
        addAttrMap={jest.fn()}
        attrMapErr={{ emptyName: false, emptyValue: false }}
        setAttrMapErr={jest.fn()}
        disabled={false}
        preset={SamlServiceProviderPreset.Unspecified}
        isGuided={false}
      />
      <Button />
    </Validation>
  );

  const attribute = {
    name: 'firstname',
    name_format: 'unspecified',
    value: 'user.spec.traits.firstname',
  };

  fireEvent.change(screen.getByPlaceholderText('attribute_name'), {
    target: { value: attribute.name },
  });

  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({
      attributeMapping: [
        { name: attribute.name, name_format: 'unspecified', value: '' },
      ],
    })
  );

  const attrValEl = screen.getByLabelText('attribute value');
  fireEvent.change(attrValEl, {
    target: { value: attribute.value },
  });
  await user.type(attrValEl, '{enter}');

  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({
      attributeMapping: [
        {
          name: '',
          name_format: 'unspecified',
          value: 'user.spec.traits.firstname',
        },
      ],
    })
  );
});

test('attribute maping subheader per preset', async () => {
  render(
    <Validation>
      <AttributeMapping
        spConfig={emptyUpsertRequest}
        setSPConfig={jest.fn()}
        addAttrMap={jest.fn()}
        attrMapErr={{ emptyName: false, emptyValue: false }}
        setAttrMapErr={jest.fn()}
        disabled={true}
        preset={SamlServiceProviderPreset.Unspecified}
        isGuided={true}
      />
    </Validation>
  );
  expect(
    screen.getByText(
      'Teleport sends username as "uid" attribute and roles as "eduPersonAffiliation" attribute',
      { exact: false }
    )
  ).toBeInTheDocument();

  render(
    <Validation>
      <AttributeMapping
        spConfig={emptyUpsertRequest}
        setSPConfig={jest.fn()}
        addAttrMap={jest.fn()}
        attrMapErr={{ emptyName: false, emptyValue: false }}
        setAttrMapErr={jest.fn()}
        disabled={true}
        preset={SamlServiceProviderPreset.GcpWorkforce}
        isGuided={true}
      />
    </Validation>
  );
  expect(
    screen.getByText(
      'An attribute named "roles" with values containing Teleport roles for user will be sent by default',
      { exact: false }
    )
  ).toBeInTheDocument();
});

test('attribute maping disabled', async () => {
  render(
    <Validation>
      <AttributeMapping
        spConfig={emptyUpsertRequest}
        setSPConfig={jest.fn()}
        addAttrMap={jest.fn()}
        attrMapErr={{ emptyName: false, emptyValue: false }}
        setAttrMapErr={jest.fn()}
        disabled={true}
        preset={SamlServiceProviderPreset.Unspecified}
        isGuided={true}
      />
    </Validation>
  );
  expect(screen.getByPlaceholderText('attribute_name')).toBeDisabled();
  expect(screen.getByLabelText('attribute value')).toBeDisabled();
  expect(screen.getByText('Add Another Attribute Mapping')).toBeDisabled();
});
