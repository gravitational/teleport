import { chakra } from '@chakra-ui/react';
import {
  forwardRef,
  useCallback,
  type ComponentProps,
  type FormEvent,
  type ForwardedRef,
  type PropsWithChildren,
} from 'react';
import {
  FormProvider,
  type FieldValues,
  type SubmitHandler,
  type UseFormReturn,
} from 'react-hook-form';

interface FormProps<
  TFieldValues extends FieldValues,
  TContext = any,
  TTransformedValues extends FieldValues | undefined = undefined,
> extends Omit<ComponentProps<'form'>, 'onSubmit'> {
  form: UseFormReturn<TFieldValues, TContext, TTransformedValues>;
  onSubmit?: TTransformedValues extends undefined
    ? SubmitHandler<TFieldValues>
    : TTransformedValues extends FieldValues
      ? SubmitHandler<TTransformedValues>
      : never;
}

export const Form = forwardRef(function Form<
  TFieldValues extends FieldValues,
  TContext = any,
  TTransformedValues extends FieldValues | undefined = undefined,
>(
  {
    children,
    form,
    onSubmit,
    ...rest
  }: PropsWithChildren<FormProps<TFieldValues, TContext, TTransformedValues>>,
  ref: ForwardedRef<HTMLElement>
) {
  const handleSubmit = useCallback(
    (event: FormEvent) => {
      if (!onSubmit) {
        return;
      }

      void form.handleSubmit(onSubmit)(event);
    },
    [form, onSubmit]
  );

  return (
    <FormProvider<TFieldValues, TContext, TTransformedValues> {...form}>
      <chakra.form {...rest} ref={ref} onSubmit={handleSubmit}>
        {children}
      </chakra.form>
    </FormProvider>
  );
});
