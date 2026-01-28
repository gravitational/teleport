import {
  useCallback,
  type ComponentProps,
  type FormEvent,
  type PropsWithChildren,
  type RefAttributes,
} from 'react';
import {
  FormProvider,
  type FieldValues,
  type SubmitHandler,
  type UseFormReturn,
} from 'react-hook-form';

// Note: this will be replaced by the new design system

interface FormProps<
  TFieldValues extends FieldValues = FieldValues,
  TContext = unknown,
  TTransformedValues = TFieldValues,
> {
  form: UseFormReturn<TFieldValues, TContext, TTransformedValues>;
  onSubmit?: SubmitHandler<
    TTransformedValues extends FieldValues ? TTransformedValues : TFieldValues
  >;
}

export function Form<
  TFieldValues extends FieldValues = FieldValues,
  TContext = unknown,
  TTransformedValues = TFieldValues,
>({
  children,
  form,
  onSubmit,
  ref,
  ...rest
}: PropsWithChildren<FormProps<TFieldValues, TContext, TTransformedValues>> &
  RefAttributes<HTMLFormElement> &
  Omit<ComponentProps<'form'>, 'onSubmit'>) {
  const handleSubmit = useCallback(
    (event: FormEvent) => {
      if (!onSubmit) {
        return;
      }

      void form.handleSubmit(onSubmit as SubmitHandler<TTransformedValues>)(
        event
      );
    },
    [form, onSubmit]
  );

  return (
    <FormProvider<TFieldValues, TContext, TTransformedValues> {...form}>
      <form {...rest} onSubmit={handleSubmit} ref={ref}>
        {children}
      </form>
    </FormProvider>
  );
}

export function NestedForm<
  TFieldValues extends FieldValues = FieldValues,
  TContext = unknown,
  TTransformedValues = TFieldValues,
>({
  children,
  form,
  ref,
  ...rest
}: PropsWithChildren<FormProps<TFieldValues, TContext, TTransformedValues>> &
  RefAttributes<HTMLDivElement> &
  ComponentProps<'div'>) {
  return (
    <FormProvider<TFieldValues, TContext, TTransformedValues> {...form}>
      <div {...rest} ref={ref}>
        {children}
      </div>
    </FormProvider>
  );
}
