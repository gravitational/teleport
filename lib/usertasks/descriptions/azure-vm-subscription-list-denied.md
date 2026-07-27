# Cannot access Azure subscriptions

Teleport could not resolve the wildcard subscription matcher because the Azure integration cannot access any subscriptions.

The UserTask's `tenant_id` and `client_id` fields identify the Microsoft Entra tenant and managed identity or service principal used by the integration. Verify that these identifiers are correct, then grant that identity a role with the following actions at the management group or subscription scope that Teleport should discover:

- `Microsoft.Compute/virtualMachines/read`
- `Microsoft.Compute/virtualMachines/runCommands/write`
- `Microsoft.Compute/virtualMachines/runCommands/read`
- `Microsoft.Compute/virtualMachineScaleSets/read`
- `Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read`
- `Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/write`
- `Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/read`
- `Microsoft.Resources/subscriptions/read`

Alternatively, replace the wildcard (`"*"`) in the DiscoveryConfig with explicit subscription IDs. The integration identity must still have the required VM discovery permissions in those subscriptions.

Azure role assignments may take several minutes to propagate. After the permissions propagate, update the DiscoveryConfig to trigger reconciliation. This task expires automatically if the issue is not observed again.
