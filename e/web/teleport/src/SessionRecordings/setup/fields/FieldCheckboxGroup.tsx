import {
  useCallback,
  useId,
  useMemo,
  type KeyboardEvent,
  type ReactNode,
} from 'react';
import {
  useController,
  useWatch,
  type FieldPath,
  type FieldValues,
} from 'react-hook-form';
import styled from 'styled-components';

import Box, { BoxProps } from 'design/Box';
import Flex from 'design/Flex';
import { Check } from 'design/Icon';
import { LabelContent, LabelInput } from 'design/LabelInput/LabelInput';
import Text from 'design/Text';
import { HelperTextLine } from 'shared/components/FieldInput/FieldInput';

export interface CheckboxOption<T extends string = string> {
  value: T;
  label: string;
  icon?: ReactNode;
}

interface FieldCheckboxGroupProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> {
  containerProps?: BoxProps;
  disabled?: boolean;
  helperText?: ReactNode;
  label?: ReactNode;
  name: TName;
  options: CheckboxOption[];
  required?: boolean;
}

export function FieldCheckboxGroup<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
  containerProps,
  disabled,
  helperText,
  label,
  name,
  options,
  required,
}: FieldCheckboxGroupProps<TFieldValues, TName>) {
  const { field, fieldState, formState } = useController<TFieldValues, TName>({
    name,
  });

  const helperTextId = useId();

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

        <Flex gap={2} flexWrap="wrap">
          {options.map(option => (
            <CheckboxOption
              key={option.value}
              disabled={isDisabled}
              name={name}
              onChange={field.onChange}
              option={option}
            />
          ))}
        </Flex>
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

interface CheckboxOptionProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> {
  disabled?: boolean;
  name: TName;
  onChange: (value: string[]) => void;
  option: CheckboxOption;
}

function CheckboxOption<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({
  disabled,
  name,
  onChange,
  option,
}: CheckboxOptionProps<TFieldValues, TName>) {
  const value = useWatch<TFieldValues, TName>({ name });
  const values = useMemo(() => (Array.isArray(value) ? value : []), [value]);

  const selected = values.includes(option.value);

  const handleSelect = useCallback(() => {
    if (selected) {
      onChange(values.filter(v => v !== option.value));
    } else {
      onChange([...values, option.value]);
    }
  }, [onChange, option.value, selected, values]);

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        handleSelect();
      }
    },
    [handleSelect]
  );

  return (
    <OptionContainer
      selected={selected}
      aria-checked={selected}
      aria-disabled={disabled}
      onClick={handleSelect}
      onKeyDown={handleKeyDown}
      role="checkbox"
      tabIndex={disabled ? -1 : 0}
    >
      <CheckboxSquare selected={selected}>
        {selected && <Check size="small" />}
      </CheckboxSquare>

      <Flex alignItems="center" gap={2}>
        {option.icon}
        <Text>{option.label}</Text>
      </Flex>
    </OptionContainer>
  );
}

const OptionContainer = styled.div<{ selected: boolean }>`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[3]}px;
  padding: 6px ${p => p.theme.space[3]}px;
  font-size: 14px;
  border: 1px solid
    ${p =>
      p.selected
        ? p.theme.colors.interactive.solid.primary.default
        : p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${p => p.theme.radii[3]}px;
  cursor: pointer;
  user-select: none;
  height: 40px;
  box-sizing: border-box;
  background: ${p =>
    p.selected ? p.theme.colors.interactive.tonal.primary[0] : 'transparent'};

  &:hover {
    background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }

  &:focus-visible {
    outline: 2px solid ${p => p.theme.colors.interactive.solid.primary.default};
    outline-offset: 2px;
  }
`;

const CheckboxSquare = styled.div<{ selected: boolean }>`
  display: flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border: 1px solid
    ${p =>
      p.selected
        ? p.theme.colors.interactive.solid.primary.default
        : p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 3px;
  background: ${p =>
    p.selected
      ? p.theme.colors.interactive.solid.primary.default
      : 'transparent'};
  color: ${p => p.theme.colors.text.primaryInverse};
`;
