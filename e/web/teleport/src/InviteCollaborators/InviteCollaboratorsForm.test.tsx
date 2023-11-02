import React from 'react';
import { fireEvent, userEvent, render, screen } from 'design/utils/testing';

import { Option } from 'shared/components/Select';
import Validation, { useValidation } from 'shared/components/Validation';

import { InviteCollaboratorsForm } from './InviteCollaboratorsForm';
import { InviteCollaboratorsFormProps, RoleOption } from './types';

function makeRole(name: string): RoleOption {
  return {
    label: name,
    value: {
      name,
      description: `${name} description`,
    },
  };
}

function makeRoles(...names: string[]): RoleOption[] {
  return names.map(makeRole);
}

function makeUserOption(name: string): Option<string> {
  return {
    label: name,
    value: name,
  };
}

describe('invite form', () => {
  let props: InviteCollaboratorsFormProps;

  const onClose = jest.fn();

  beforeEach(() => {
    props = {
      users: new Set(['alice@example.com']),
      roles: makeRoles('foo', 'bar', 'baz'),
      recipientsValue: [],
      setRecipientsValue: jest.fn(),
      selectedRoles: [],
      setSelectedRoles: jest.fn(),
      onClose,
      hidden: false,
    };
  });

  test('lists all roles', async () => {
    render(
      <Validation>
        <InviteCollaboratorsForm {...props} />
      </Validation>
    );

    expect(screen.getByLabelText('Recipients')).toBeInTheDocument();

    const rolesSelect = screen.getByLabelText('User Roles');
    expect(rolesSelect).toBeInTheDocument();

    await userEvent.click(rolesSelect);

    expect(screen.getByText('foo')).toBeInTheDocument();
    expect(screen.getByText('bar')).toBeInTheDocument();
    expect(screen.getByText('baz')).toBeInTheDocument();
  });

  test('requests to close on escape press in recipients field', async () => {
    render(
      <Validation>
        <InviteCollaboratorsForm {...props} />
      </Validation>
    );

    expect(screen.getByLabelText('Recipients')).toBeInTheDocument();

    const recipients = screen.getByLabelText('Recipients');
    expect(recipients).toBeInTheDocument();

    expect(onClose.mock.calls).toHaveLength(0);

    await userEvent.click(recipients);
    await userEvent.keyboard('{Escape}');

    expect(onClose.mock.calls).toHaveLength(1);
  });

  test('succeeds with valid data', async () => {
    props.recipientsValue.push(makeUserOption('bob@example.com'));
    props.selectedRoles.push(props.roles[0]);

    let validator = null;
    const Button = () => {
      validator = useValidation();
      return <button role="button" onClick={() => validator.validate()} />;
    };

    render(
      <Validation>
        <>
          <InviteCollaboratorsForm {...props} />
          <Button />
        </>
      </Validation>
    );

    fireEvent.click(screen.getByRole('button'));

    expect(validator.valid).toBe(true);
  });

  test('requires some data', async () => {
    let validator = null;
    const Button = () => {
      validator = useValidation();
      return <button role="button" onClick={() => validator.validate()} />;
    };

    render(
      <Validation>
        <>
          <InviteCollaboratorsForm {...props} />
          <Button />
        </>
      </Validation>
    );

    await userEvent.click(screen.getByRole('button'));

    expect(validator.valid).toBe(false);
    expect(
      screen.getByText('At least one address is required')
    ).toBeInTheDocument();
    expect(
      screen.getByText('At least one role is required')
    ).toBeInTheDocument();
  });

  test('does not allow duplicate users', async () => {
    props.recipientsValue.push(makeUserOption('alice@example.com'));

    let validator = null;
    const Button = () => {
      validator = useValidation();
      return <button role="button" onClick={() => validator.validate()} />;
    };

    render(
      <Validation>
        <>
          <InviteCollaboratorsForm {...props} />
          <Button />
        </>
      </Validation>
    );

    await userEvent.click(screen.getByRole('button'));

    expect(validator.valid).toBe(false);
    expect(
      screen.getByText('User already exists: alice@example.com')
    ).toBeInTheDocument();
  });

  test('requires an email-like username', async () => {
    props.recipientsValue.push(makeUserOption('alice'));

    let validator = null;
    const Button = () => {
      validator = useValidation();
      return <button role="button" onClick={() => validator.validate()} />;
    };

    render(
      <Validation>
        <>
          <InviteCollaboratorsForm {...props} />
          <Button />
        </>
      </Validation>
    );

    await userEvent.click(screen.getByRole('button'));

    expect(validator.valid).toBe(false);
    expect(screen.getByText('Email is invalid: alice')).toBeInTheDocument();
  });
});
