import { Field } from '@ark-ui/react';
import {
  Box,
  createSlotRecipeContext,
  Flex,
  useSlotRecipe,
  type InputProps as ChakraInputProps,
  type IconProps,
} from '@chakra-ui/react';
import { forwardRef, type RefAttributes } from 'react';

import { WarningCircleIcon } from '../../icons/WarningCircle';

const {
  withContext,
  useStyles: useInputStyles,
  StylesProvider,
} = createSlotRecipeContext({ key: 'input2' });

const StyledField = withContext<HTMLInputElement, InputProps>(
  Field.Input,
  'field',
  { forwardAsChild: true }
);

export interface InputProps extends ChakraInputProps {
  hasError?: boolean;
}

export const Input = forwardRef<
  HTMLInputElement,
  InputProps & RefAttributes<HTMLInputElement>
>(function Input(props, ref) {
  const recipe = useSlotRecipe({ key: 'input2' });

  const [recipeProps, restProps] = recipe.splitVariantProps(props);

  const { hasError, ...rest } = restProps;

  const styles = recipe(recipeProps);

  return (
    <StylesProvider value={styles}>
      <Box css={styles.container}>
        <StyledField data-invalid={hasError} {...rest} ref={ref} />

        {hasError && <ErrorIcon />}
      </Box>
    </StylesProvider>
  );
});

function ErrorIcon(props: IconProps) {
  const { icon } = useInputStyles();

  return (
    <Flex css={icon}>
      <WarningCircleIcon {...props} />
    </Flex>
  );
}
