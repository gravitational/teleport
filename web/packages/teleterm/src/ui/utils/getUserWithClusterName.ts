export function getUserWithClusterName(params: {
  userName?: string;
  clusterName: string;
}): string | undefined {
  return params.userName
    ? `${params.userName}@${params.clusterName}`
    : params.clusterName;
}
