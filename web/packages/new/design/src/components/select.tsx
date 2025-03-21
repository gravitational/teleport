import {
  Select as ChakraSelect,
  type ChakraStylesConfig,
  type GroupBase,
  type Props,
} from 'chakra-react-select';
import { useMemo } from 'react';

function createStyles<
  Option,
  IsMulti extends boolean,
  Group extends GroupBase<Option>,
>(): ChakraStylesConfig<Option, IsMulti, Group> {
  return {
    option: (provided, state) => ({
      ...provided,
      bg: state.isSelected ? 'interactive.tonal.neutral.1' : 'levels.surface',
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

export function Select<
  Option = unknown,
  IsMulti extends boolean = false,
  Group extends GroupBase<Option> = GroupBase<Option>,
>(props: Props<Option, IsMulti, Group>) {
  const styles = useMemo(() => createStyles<Option, IsMulti, Group>(), []);

  return (
    <ChakraSelect<Option, IsMulti, Group>
      {...props}
      chakraStyles={styles}
      selectedOptionColorPalette="interactive.tonal.neutral"
    />
  );
}
