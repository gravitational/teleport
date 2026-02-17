import { useCallback, useId, type ReactNode, type RefAttributes } from 'react';
import { useController } from 'react-hook-form';
import { Props, type GroupBase, type SelectInstance } from 'react-select';

import Box, { BoxProps } from 'design/Box';
import Flex from 'design/Flex';
import { LabelContent, LabelInput } from 'design/LabelInput/LabelInput';
import { HelperTextLine } from 'shared/components/FieldInput';
import Select from 'shared/components/Select';
import { type Option } from 'shared/components/Select/types';

interface FieldSelectProps<
  Opt extends object = Option,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
> extends Omit<Props<Opt, false, Group>, 'onChange'> {
  containerProps?: BoxProps;
  disabled?: boolean;
  helperText?: ReactNode;
  label?: string;
  labelButton?: ReactNode;
  name: string;
  onChange?: (value: Opt | undefined) => void;
  required?: boolean;
}

export function FieldSelect<
  Opt extends object = Option,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>({
  containerProps,
  disabled,
  helperText,
  label,
  labelButton,
  name,
  onChange,
  required,
  ...rest
}: FieldSelectProps<Opt, Group> &
  RefAttributes<SelectInstance<Opt, false, Group>>) {
  const { field, fieldState, formState } = useController({
    name,
  });

  const helperTextId = useId();

  const isDisabled = formState.isSubmitting || formState.disabled || disabled;

  const handleChange = useCallback(
    (newValue: Opt | null) => {
      if (!newValue) {
        field.onChange(undefined);
        onChange?.(undefined);

        return;
      }

      if ('value' in newValue) {
        field.onChange(newValue.value);
        onChange?.(newValue);
      }
    },
    [field, onChange]
  );

  const value =
    rest.options
      ?.flatMap(item => ('options' in item ? item.options : [item]))
      .find(option => 'value' in option && option.value === field.value) ??
    null;

  return (
    <Box
      opacity={isDisabled ? 0.5 : 1}
      style={{ pointerEvents: isDisabled ? 'none' : 'auto' }}
      {...containerProps}
    >
      <LabelInput mb={0}>
        <Flex justifyContent="space-between" alignItems="center" mb={1}>
          <LabelContent required={required}>{label}</LabelContent>

          {labelButton}
        </Flex>

        <Select<Opt, false, Group>
          {...rest}
          {...field}
          isDisabled={isDisabled}
          onChange={handleChange}
          value={value}
        />
      </LabelInput>

      <HelperTextLine
        hasError={fieldState.invalid}
        helperTextId={helperTextId}
        helperText={helperText}
        errorMessage={fieldState.error?.message}
      />
    </Box>
  );
}
