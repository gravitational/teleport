import React, {
  HTMLInputAutoCompleteAttribute,
  useId,
  type RefAttributes,
} from 'react';
import {
  useController,
  type FieldPath,
  type FieldValues,
} from 'react-hook-form';

import Box, { BoxProps } from 'design/Box';
import Input, { InputSize, type InputType } from 'design/Input';
import { LabelContent, LabelInput } from 'design/LabelInput/LabelInput';
import { HelperTextLine } from 'shared/components/FieldInput/FieldInput';

import { useMergeRefs } from './useMergeRefs';

// Note: this will be replaced by the new design system

interface FieldInputProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> {
  allow1p?: boolean; // allow 1Password autocomplete
  autoFocus?: boolean;
  autoComplete?: HTMLInputAutoCompleteAttribute;
  containerProps?: BoxProps;
  disabled?: boolean;
  helperText?: React.ReactNode;
  label?: React.ReactNode;
  name: TName;
  placeholder?: string;
  required?: boolean;
  size?: InputSize;
  type?: InputType;
}

export function FieldInput<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
  TTransformedValues = TFieldValues,
>({
  allow1p,
  containerProps,
  disabled,
  helperText,
  label,
  name,
  placeholder,
  ref,
  required,
  size,
  type,
  ...rest
}: FieldInputProps<TFieldValues, TName> & RefAttributes<HTMLInputElement>) {
  const { field, fieldState, formState } = useController<
    TFieldValues,
    TName,
    TTransformedValues
  >({
    name,
  });

  const helperTextId = useId();

  const inputRef = useMergeRefs<HTMLInputElement>(ref, field.ref);

  const isDisabled = formState.isSubmitting || formState.disabled || disabled;

  return (
    <Box
      opacity={isDisabled ? 0.5 : 1}
      style={{ pointerEvents: isDisabled ? 'none' : 'auto' }}
      {...containerProps}
    >
      <LabelInput mb={0}>
        <LabelContent required={required} mb={1}>
          {label}
        </LabelContent>

        <Input
          aria-invalid={fieldState.invalid}
          aria-describedby={helperTextId}
          data-1p-ignore={!allow1p}
          disabled={isDisabled}
          hasError={fieldState.invalid}
          placeholder={placeholder}
          ref={inputRef}
          required={required}
          size={size}
          type={type}
          {...rest}
          {...field}
          value={field.value ?? ''}
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

export function FieldInputPassword<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
  TTransformedValues = TFieldValues,
>(
  props: Omit<FieldInputProps<TFieldValues, TName>, 'type'> &
    RefAttributes<HTMLInputElement>
) {
  return (
    <FieldInput<TFieldValues, TName, TTransformedValues>
      type="password"
      {...props}
    />
  );
}
