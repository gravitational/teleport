import { forwardRef, type ReactNode, type RefAttributes } from 'react';
import { useController } from 'react-hook-form';

import { Field, Input } from '../index';
import type { InputProps } from './Input';

interface FieldInputProps extends InputProps {
  label?: string;
  name: string;
  helperText?: ReactNode;
  required?: boolean;
}

export const FieldInput = forwardRef<
  HTMLInputElement,
  FieldInputProps & RefAttributes<HTMLInputElement>
>(function FieldInput({ helperText, name, label, required, ...rest }, ref) {
  const {
    field,
    fieldState,
    formState: { isSubmitting },
  } = useController({
    name,
  });

  return (
    <Field.Root invalid={fieldState.invalid} required={required}>
      {label && <Field.Label>{label}</Field.Label>}

      <Input
        hasError={fieldState.invalid}
        disabled={isSubmitting}
        {...rest}
        {...field}
        ref={ref}
        value={field.value ?? ''}
      />

      <Field.ErrorText>{fieldState.error?.message}</Field.ErrorText>

      {helperText && <Field.HelperText>{helperText}</Field.HelperText>}
    </Field.Root>
  );
});

export const FieldInputPassword = forwardRef<
  HTMLInputElement,
  FieldInputProps & RefAttributes<HTMLInputElement>
>(function FieldInputPassword(props, ref) {
  return (
    <FieldInput {...props} placeholder="Password" type="password" ref={ref} />
  );
});
