import type { ConditionalValue } from '@chakra-ui/react';

export type ExtractValueType<T> =
  T extends ConditionalValue<infer U> ? U : never;
