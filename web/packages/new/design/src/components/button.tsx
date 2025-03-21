import { Button, type ButtonProps } from '@chakra-ui/react';
import { forwardRef } from 'react';

export const ButtonPrimary = forwardRef<HTMLButtonElement, ButtonProps>(
  function ButtonPrimary(props, ref) {
    return <Button variant="filled" intent="primary" ref={ref} {...props} />;
  }
);

export const ButtonSecondary = forwardRef<HTMLButtonElement, ButtonProps>(
  function ButtonSecondary(props, ref) {
    return <Button variant="filled" intent="neutral" ref={ref} {...props} />;
  }
);

export const ButtonBorder = forwardRef<HTMLButtonElement, ButtonProps>(
  function ButtonBorder(props, ref) {
    return <Button variant="border" intent="neutral" ref={ref} {...props} />;
  }
);

export const ButtonWarning = forwardRef<HTMLButtonElement, ButtonProps>(
  function ButtonWarning(props, ref) {
    return <Button variant="filled" intent="danger" ref={ref} {...props} />;
  }
);

export const ButtonWarningBorder = forwardRef<HTMLButtonElement, ButtonProps>(
  function ButtonWarningBorder(props, ref) {
    return <Button variant="border" intent="danger" ref={ref} {...props} />;
  }
);

export const ButtonText = forwardRef<HTMLButtonElement, ButtonProps>(
  function ButtonText(props, ref) {
    return <Button variant="minimal" intent="neutral" ref={ref} {...props} />;
  }
);
