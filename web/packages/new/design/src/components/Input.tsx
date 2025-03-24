import { Field } from '@ark-ui/react';
import {
  Box,
  createSlotRecipeContext,
  useSlotRecipe,
  type InputProps as ChakraInputProps,
  type IconProps,
} from '@chakra-ui/react';
import { forwardRef, type ReactNode, type RefAttributes } from 'react';

import { WarningCircleIcon } from '../icons/WarningCircle';

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
  icon?: ReactNode;
}

export const Input = forwardRef<
  HTMLInputElement,
  InputProps & RefAttributes<HTMLInputElement>
>(function Input(props, ref) {
  const recipe = useSlotRecipe({ key: 'input2' });

  const [recipeProps, restProps] = recipe.splitVariantProps(props);

  const { hasError, icon, ...rest } = restProps;

  const styles = recipe(recipeProps);

  const paddingLeft = icon ? 10 : undefined;
  const paddingRight = hasError ? 10 : undefined;

  return (
    <StylesProvider value={styles}>
      <Box css={styles.container}>
        {icon && (
          <Box css={styles.icon} left={3}>
            {icon}
          </Box>
        )}

        <StyledField
          data-invalid={hasError ? 'true' : undefined}
          {...rest}
          pl={paddingLeft}
          pr={paddingRight}
          ref={ref}
        />

        {hasError && <ErrorIcon />}
      </Box>
    </StylesProvider>
  );
});

function ErrorIcon(props: IconProps) {
  const { icon } = useInputStyles();

  console.log('icon', icon);

  return (
    <Box css={icon} right={3} color="interactive.solid.danger.default">
      <WarningCircleIcon {...props} />
    </Box>
  );
}
