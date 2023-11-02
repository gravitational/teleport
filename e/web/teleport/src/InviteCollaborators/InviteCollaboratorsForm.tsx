import React, {
  useState,
  KeyboardEventHandler,
  KeyboardEvent,
  FocusEventHandler,
} from 'react';
import { useTheme } from 'styled-components';
import { Text } from 'design';
import FieldSelect, {
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import {
  requiredField,
  requiredEmailLike,
} from 'shared/components/Validation/rules';

import { StylesConfig, OptionProps } from 'react-select';

import { RoleOption, InviteCollaboratorsFormProps } from './types';
import {
  requiredAllUsersDoNotExist,
  requiredNoDuplicateUsers,
  requiredAllEmailLike,
  requiredAll,
  requiredMaxDuplicates,
} from './rules';

/**
 * `react-select` style configuration for the recipients field.
 */
const recipientStyles = (
  users: Set<string>,
  enteredUsers: Option[],
  theme
): StylesConfig<Option> => {
  return {
    multiValue: (styles, { data }) => {
      const { valid } = requiredAll(
        requiredEmailLike,
        requiredMaxDuplicates(users, 0),
        requiredMaxDuplicates(
          enteredUsers.map(u => u.value),
          1
        )
      )(data.label)();
      const errorStyle = valid
        ? null
        : {
            borderColor: theme.colors.error.main,
            borderWidth: '1px',
            borderStyle: 'solid',
          };

      return {
        ...styles,
        ...errorStyle,
        borderRadius: '1em',
      };
    },
  };
};

/**
 * `react-select` component overrides for the recipients field. This disables
 * the dropdown indicator as there are no preexisting recipients to pick from.
 */
const recipientsComponents = {
  DropdownIndicator: null,
};

/**
 * `react-select` style overrides for the role selector.
 */
const roleStyles = (theme): StylesConfig<Option> => {
  return {
    multiValue: styles => ({
      ...styles,
      borderRadius: '1em',
    }),
    menuList: styles => ({
      ...styles,
      backgroundColor: theme.colors.levels.elevated,
    }),
  };
};

/**
 * A replacement `react-select` component to show both the name and description
 * of a role.
 */
const RoleOptionComponent = (props: OptionProps<RoleOption>) => {
  const {
    getStyles,
    className,
    cx,
    innerRef,
    isDisabled,
    isFocused,
    isSelected,
    innerProps,
    value,
  } = props;

  const theme = useTheme();

  return (
    <div
      ref={innerRef}
      css={getStyles('option', props)}
      className={cx(
        {
          option: true,
          'option--is-disabled': isDisabled,
          'option--is-focused': isFocused,
          'option--is-selected': isSelected,
        },
        className
      )}
      {...innerProps}
    >
      <Text
        color={theme.colors.text.main}
        css={{ 'text-transform': 'capitalize' }}
      >
        {value.name}
      </Text>
      <Text color={theme.colors.text.muted}>{value.description}</Text>
    </div>
  );
};

/**
 * `react-select` component overrides for the role selector, using the
 * replacement `RoleOptionComponent`.
 */
const rolesComponents = {
  Option: RoleOptionComponent,
};

export function InviteCollaboratorsForm({
  users,
  roles,
  recipientsValue,
  setRecipientsValue,
  selectedRoles,
  setSelectedRoles,
  onClose,
  hidden,
}: InviteCollaboratorsFormProps) {
  const theme = useTheme();

  const [recipientsInput, setRecipientsInput] = useState('');

  function onChangeRoles(roles: RoleOption[] = []) {
    setSelectedRoles(roles);
  }

  const createRecipientOption = (label: string) => ({
    label,
    value: label,
  });

  const handleRecipientsKeyDown: KeyboardEventHandler = (
    event: KeyboardEvent
  ) => {
    if (event.key == 'Escape') {
      if (onClose) {
        onClose();
      }
      return;
    }

    if (!recipientsInput) {
      return;
    }

    switch (event.key) {
      case 'Enter':
      case 'Tab':
      case ' ':
      case ',':
        setRecipientsValue(prev => [
          ...prev,
          createRecipientOption(recipientsInput),
        ]);
        setRecipientsInput('');
        event.preventDefault();
    }
  };

  const handleRecipientsValueChange = (value?: Option[]) => {
    setRecipientsValue(value || []);
  };

  const handleRecipientsBlur: FocusEventHandler = () => {
    if (recipientsInput.length > 0) {
      setRecipientsValue(prev => [
        ...prev,
        createRecipientOption(recipientsInput),
      ]);
    }
  };

  return (
    <div style={{ visibility: hidden ? 'hidden' : 'visible' }}>
      <FieldSelectCreatable
        components={recipientsComponents}
        label="Recipients"
        rule={requiredAll(
          requiredAllEmailLike,
          requiredAllUsersDoNotExist(users),
          requiredNoDuplicateUsers
        )}
        placeholder="Enter email addresses"
        autoFocus
        isClearable
        isMulti
        isSearchable
        menuIsOpen={false}
        inputValue={recipientsInput}
        value={recipientsValue}
        onInputChange={v => setRecipientsInput(v)}
        onChange={v => handleRecipientsValueChange(v as Option[] | null)}
        onKeyDown={handleRecipientsKeyDown}
        onBlur={handleRecipientsBlur}
        stylesConfig={recipientStyles(users, recipientsValue, theme)}
        inputId="recipients"
      />
      <FieldSelect
        components={rolesComponents}
        menuPosition="fixed"
        label="User Roles"
        rule={requiredField('At least one role is required')}
        placeholder="Click to select roles"
        isSearchable
        isMulti
        isSimpleValue
        isClearable={false}
        value={selectedRoles}
        onChange={values => onChangeRoles(values as RoleOption[])}
        options={roles}
        elevated={true}
        stylesConfig={roleStyles(theme)}
        inputId="roles"
      />
    </div>
  );
}
