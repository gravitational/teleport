import {
  Select as ChakraSelect,
  type ChakraStylesConfig,
  type GroupBase,
  type Props,
  type SelectComponent,
  type SelectInstance,
} from 'chakra-react-select';
import { forwardRef, useCallback, useMemo, type ForwardedRef } from 'react';

export type { SelectInstance };

function createStyles<
  Option,
  IsMulti extends boolean,
  Group extends GroupBase<Option>,
>(): ChakraStylesConfig<Option, IsMulti, Group> {
  return {
    option: (provided, state) => ({
      ...provided,
      bg: state.isSelected
        ? 'interactive.tonal.neutral.1'
        : state.isFocused
          ? 'interactive.tonal.neutral.0'
          : 'levels.surface',
      color: 'text.main',
      fontSize: 'md',
      fontWeight: state.isSelected ? 'bold' : 'normal',
      px: 4,
      py: 3,
      cursor: 'pointer',
      _focusVisible: {
        bg: 'interactive.tonal.neutral.0',
        _hover: {
          bg: 'interactive.tonal.neutral.0',
        },
      },
      _hover: {
        bg: state.isSelected
          ? 'interactive.tonal.neutral.1'
          : 'interactive.tonal.neutral.0',
      },
    }),
    control: provided => ({
      ...provided,
      cursor: 'pointer',
      _hover: {
        borderColor: 'text.muted',
      },
      _focus: {
        borderColor: 'interactive.solid.primary.default',
        outline: 0,
      },
    }),
    menu: provided => ({
      ...provided,
      px: 0,
    }),
    menuList: provided => ({
      ...provided,
      px: 0,
    }),
    singleValue: provided => ({
      ...provided,
      color: {
        _dark: 'text.main',
      },
    }),
  };
}

export const Select = forwardRef(function Select<
  Option,
  IsMulti extends boolean,
  Group extends GroupBase<Option>,
>(
  props: Props<Option, IsMulti, Group>,
  forwardedRef:
    | ((instance: SelectInstance<Option, IsMulti, Group> | null) => void)
    | ForwardedRef<SelectInstance<Option, IsMulti, Group> | null>
    | null
) {
  const styles = useMemo(() => createStyles<Option, IsMulti, Group>(), []);

  const ref = useCallback(
    (instance: SelectInstance<Option, IsMulti, Group> | null) => {
      if (typeof forwardedRef === 'function') {
        forwardedRef(instance);
      } else if (forwardedRef && 'current' in forwardedRef) {
        forwardedRef.current = instance;
      }
    },
    [forwardedRef]
  );

  return (
    <ChakraSelect<Option, IsMulti, Group>
      {...props}
      ref={ref}
      chakraStyles={styles}
      selectedOptionColorPalette="interactive.tonal.neutral"
    />
  );
}) as SelectComponent;
