# Cannot access Azure subscriptions

Teleport could not resolve the wildcard subscription matcher because the Azure integration cannot access any subscriptions.

Verify that the integration uses the intended Azure tenant and managed identity or service principal. Grant that identity the Azure VM discovery role on each subscription that Teleport should discover.

Alternatively, replace the wildcard (`"*"`) in the DiscoveryConfig with explicit subscription IDs. The integration identity must still have the required VM discovery permissions in those subscriptions.

Azure role assignments may take several minutes to propagate. Teleport retries discovery periodically, and this task will expire automatically after subscription discovery succeeds.
