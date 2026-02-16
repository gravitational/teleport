import {
  createMutationHook,
  createQueryHook,
} from 'teleport/services/queryHelpers';

import {
  createInferenceModel,
  createInferencePolicy,
  createInferenceSecret,
  deleteInferenceModel,
  deleteInferencePolicy,
  deleteInferenceSecret,
  getInferenceModel,
  getInferencePolicy,
  getInferenceSecret,
  listInferenceModels,
  listInferencePolicies,
  listInferenceSecrets,
  testInferenceModel,
  updateInferenceModel,
  updateInferencePolicy,
  updateInferenceSecret,
} from './inference';

export const {
  createQueryKey: listInferencePoliciesQueryKey,
  useInfiniteQuery: useInfiniteListInferencePolicies,
  useQuery: useListInferencePolicies,
} = createQueryHook(
  ['inference', 'policies'],
  listInferencePolicies,
  (pageParam: string, variables) => ({
    ...variables,
    startKey: pageParam,
  })
);

export const {
  createQueryKey: listInferenceModelsQueryKey,
  useInfiniteQuery: useInfiniteListInferenceModels,
  useQuery: useListInferenceModels,
} = createQueryHook(
  ['inference', 'models'],
  listInferenceModels,
  (pageParam: string, variables) => ({
    ...variables,
    startKey: pageParam,
  })
);

export const {
  createQueryKey: listInferenceSecretsQueryKey,
  useInfiniteQuery: useInfiniteListInferenceSecrets,
  useQuery: useListInferenceSecrets,
} = createQueryHook(
  ['inference', 'secrets'],
  listInferenceSecrets,
  (pageParam: string, variables) => ({
    ...variables,
    startKey: pageParam,
  })
);

export const {
  createQueryKey: getInferenceModelQueryKey,
  useSuspenseQuery: useSuspenseGetInferenceModel,
} = createQueryHook(['inference', 'model'], getInferenceModel);

export const { useSuspenseQuery: useSuspenseGetInferencePolicy } =
  createQueryHook(['inference', 'policy'], getInferencePolicy);

export const { useSuspenseQuery: useSuspenseGetInferenceSecret } =
  createQueryHook(['inference', 'secret'], getInferenceSecret);

export const useDeleteInferenceModel = createMutationHook(deleteInferenceModel);
export const useDeleteInferencePolicy = createMutationHook(
  deleteInferencePolicy
);
export const useDeleteInferenceSecret = createMutationHook(
  deleteInferenceSecret
);
export const useUpdateInferenceModel = createMutationHook(updateInferenceModel);
export const useUpdateInferencePolicy = createMutationHook(
  updateInferencePolicy
);
export const useUpdateInferenceSecret = createMutationHook(
  updateInferenceSecret
);
export const useTestInferenceModel = createMutationHook(testInferenceModel);
export const useCreateInferenceModel = createMutationHook(createInferenceModel);
export const useCreateInferencePolicy = createMutationHook(
  createInferencePolicy
);
export const useCreateInferenceSecret = createMutationHook(
  createInferenceSecret
);
