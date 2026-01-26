import { KubernetesResourceAccess } from 'teleport/services/resources';

// TODO: we should make KubernetesResource required fields

/**
 * Identities users can assume and kube resources allowed
 * when connecting a kube cluster.
 *
 * Derived from KubernetesResourceAccess with irrelevant fields
 * omitted and relevant fields required.
 */
export type KubeIdentities = Required<
  Omit<KubernetesResourceAccess, 'kubernetes_labels'>
>;

export const emptyKubeIdentities = (): KubeIdentities => ({
  kubernetes_groups: [],
  kubernetes_resources: [],
  kubernetes_users: [],
});

/**
 * Array for iteration in switch statements.
 */
export const kubeIdentities = Object.keys(emptyKubeIdentities()) as Array<
  keyof KubeIdentities
>;
