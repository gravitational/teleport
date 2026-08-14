import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import type {
  CreateInferenceModelVariables,
  CreateInferencePolicyVariables,
  CreateInferenceSecretVariables,
  DeleteInferenceModelVariables,
  DeleteInferencePolicyVariables,
  DeleteInferenceSecretVariables,
  GetInferenceModelVariables,
  GetInferencePolicyVariables,
  GetInferenceSecretVariables,
  InferenceModel,
  InferencePolicy,
  InferenceSecret,
  ListInferenceModelsResponse,
  ListInferenceModelsVariables,
  ListInferencePoliciesResponse,
  ListInferencePoliciesVariables,
  ListInferenceSecretsResponse,
  ListInferenceSecretsVariables,
  TestInferenceModelResponse,
  TestInferenceModelVariables,
  UpdateInferenceModelVariables,
  UpdateInferencePolicyVariables,
  UpdateInferenceSecretVariables,
} from './types';

export function listInferencePolicies({
  clusterId,
  limit,
  startKey,
}: ListInferencePoliciesVariables): Promise<ListInferencePoliciesResponse> {
  return api.get(cfg.getListInferencePoliciesUrl(clusterId, limit, startKey));
}

export function listInferenceModels({
  clusterId,
  limit,
  startKey,
}: ListInferenceModelsVariables): Promise<ListInferenceModelsResponse> {
  return api.get(cfg.getListInferenceModelsUrl(clusterId, limit, startKey));
}

export function listInferenceSecrets({
  clusterId,
  limit,
  startKey,
}: ListInferenceSecretsVariables): Promise<ListInferenceSecretsResponse> {
  return api.get(cfg.getListInferenceSecretsUrl(clusterId, limit, startKey));
}

export function testInferenceModel({
  clusterId,
  request,
}: TestInferenceModelVariables): Promise<TestInferenceModelResponse> {
  return api.post(cfg.getTestInferenceModelUrl(clusterId), request);
}

export function createInferencePolicy({
  clusterId,
  policy,
}: CreateInferencePolicyVariables): Promise<InferencePolicy> {
  return api.post(cfg.getInferencePoliciesUrl(clusterId), policy);
}

export function createInferenceModel({
  clusterId,
  model,
}: CreateInferenceModelVariables): Promise<InferenceModel> {
  return api.post(cfg.getInferenceModelsUrl(clusterId), model);
}

export function createInferenceSecret({
  clusterId,
  secret,
}: CreateInferenceSecretVariables): Promise<InferenceSecret> {
  return api.post(cfg.getInferenceSecretsUrl(clusterId), secret);
}

export function updateInferencePolicy({
  clusterId,
  name,
  policy,
}: UpdateInferencePolicyVariables): Promise<InferencePolicy> {
  return api.put(cfg.getInferencePolicyUrl(clusterId, name), policy);
}

export function updateInferenceModel({
  clusterId,
  name,
  model,
}: UpdateInferenceModelVariables): Promise<InferenceModel> {
  return api.put(cfg.getInferenceModelUrl(clusterId, name), model);
}

export function updateInferenceSecret({
  clusterId,
  name,
  secret,
}: UpdateInferenceSecretVariables): Promise<InferenceSecret> {
  return api.put(cfg.getInferenceSecretUrl(clusterId, name), secret);
}

export function deleteInferencePolicy({
  clusterId,
  name,
}: DeleteInferencePolicyVariables): Promise<void> {
  return api.deleteWithOptions(cfg.getInferencePolicyUrl(clusterId, name));
}

export function deleteInferenceModel({
  clusterId,
  name,
}: DeleteInferenceModelVariables): Promise<void> {
  return api.deleteWithOptions(cfg.getInferenceModelUrl(clusterId, name));
}

export function deleteInferenceSecret({
  clusterId,
  name,
}: DeleteInferenceSecretVariables): Promise<void> {
  return api.deleteWithOptions(cfg.getInferenceSecretUrl(clusterId, name));
}

export function getInferenceModel({
  clusterId,
  name,
}: GetInferenceModelVariables): Promise<InferenceModel> {
  return api.get(cfg.getInferenceModelUrl(clusterId, name));
}

export function getInferencePolicy({
  clusterId,
  name,
}: GetInferencePolicyVariables): Promise<InferencePolicy> {
  return api.get(cfg.getInferencePolicyUrl(clusterId, name));
}

export function getInferenceSecret({
  clusterId,
  name,
}: GetInferenceSecretVariables): Promise<InferenceSecret> {
  return api.get(cfg.getInferenceSecretUrl(clusterId, name));
}
