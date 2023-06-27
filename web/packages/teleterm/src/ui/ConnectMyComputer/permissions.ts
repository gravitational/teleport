import { Cluster } from 'teleterm/services/tshd/types';
import { ConfigService } from 'teleterm/services/config';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { routing } from 'teleterm/ui/uri';

/**
 * Checks if Connect My Computer can be used for the given root cluster.
 *
 * The root cluster is required because `loggedInUser` and `features` are not fully defined for leaves.
 * */
export function isConnectMyComputerPermittedForRootCluster(
  rootCluster: Cluster,
  configService: ConfigService,
  runtimeSettings: RuntimeSettings
): boolean {
  if (routing.ensureRootClusterUri(rootCluster.uri) !== rootCluster.uri) {
    throw new Error(`${rootCluster.uri} is not a root cluster`);
  }

  const isUnix =
    runtimeSettings.platform === 'darwin' ||
    runtimeSettings.platform === 'linux';

  return (
    isUnix &&
    rootCluster.loggedInUser?.acl?.tokens.create &&
    rootCluster.features?.isUsageBasedBilling &&
    configService.get('feature.connectMyComputer').value
  );
}
