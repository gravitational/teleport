# Missing Run Command permissions
Teleport uses Azure VM Run Command to install the Teleport agent during auto enrollment.
The Azure identity used by this integration does not have the permissions required to execute commands on this VM.

To fix this, grant the identity a role that includes the following permissions:
- `Microsoft.Compute/virtualMachines/runCommand/action`
- `Microsoft.Compute/virtualMachines/runCommands/read`
- `Microsoft.Compute/virtualMachines/runCommands/write`
- `Microsoft.Compute/virtualMachines/runCommands/delete`

You can assign these permissions at the Subscription level for broad access, or limit the scope to the Resource Group.