import { Field } from '@chakra-ui/react';
import type {
  GroupBase,
  OnChangeValue,
  Props,
  SelectInstance,
} from 'chakra-react-select';
import {
  forwardRef,
  useCallback,
  type ForwardedRef,
  type ReactElement,
  type ReactNode,
  type RefAttributes,
} from 'react';
import { useController } from 'react-hook-form';

import { Select } from './Select';

interface FieldSelectProps<
  Option = unknown,
  IsMulti extends boolean = boolean,
  Group extends GroupBase<Option> = GroupBase<Option>,
> extends Props<Option, IsMulti, Group> {
  label?: string;
  name: string;
  helperText?: ReactNode;
  required?: boolean;
}

type FieldSelectComponent = <
  Option = unknown,
  IsMulti extends boolean = false,
  Group extends GroupBase<Option> = GroupBase<Option>,
>(
  props: FieldSelectProps<Option, IsMulti, Group> &
    RefAttributes<SelectInstance<Option, IsMulti, Group>>
) => ReactElement;

export const FieldSelect = forwardRef(function FieldSelect<
  Option extends object,
  IsMulti extends boolean,
  Group extends GroupBase<Option>,
>(
  {
    helperText,
    name,
    label,
    required,
    ...rest
  }: FieldSelectProps<Option, IsMulti, Group>,
  forwardedRef:
    | ((instance: SelectInstance<Option, IsMulti, Group> | null) => void)
    | ForwardedRef<SelectInstance<Option, IsMulti, Group> | null>
    | null
) {
  const { field, fieldState } = useController({
    name,
  });

  const handleChange = useCallback(
    (newValue: OnChangeValue<Option, IsMulti>) => {
      if (!newValue) {
        field.onChange(undefined);

        return;
      }

      if ('value' in newValue) {
        field.onChange(newValue.value);

        return;
      }

      if (Array.isArray(newValue)) {
        field.onChange(newValue.map(option => option.value));

        return;
      }
    },
    [field]
  );

  const value = rest.options?.find(
    option => 'value' in option && option.value === field.value
  ) as Option | undefined;

  return (
    <Field.Root invalid={fieldState.invalid} required={required}>
      {label && <Field.Label>{label}</Field.Label>}

      <Select<Option, IsMulti, Group>
        {...rest}
        {...field}
        onChange={handleChange}
        ref={forwardedRef}
        value={value}
      />

      <Field.ErrorText>{fieldState.error?.message}</Field.ErrorText>

      {helperText && <Field.HelperText>{helperText}</Field.HelperText>}
    </Field.Root>
  );
}) as FieldSelectComponent;
