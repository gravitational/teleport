# VM agent not available

Teleport could not reach the Azure VM agent to run the enrollment command on this VM.
Azure reported that extension operations are disallowed.

This usually means one of the following:

**VM agent not installed or unhealthy**

Ensure the Azure VM agent is installed and running.

See Azure documentation for details: [Azure Linux VM agent](https://learn.microsoft.com/azure/virtual-machines/extensions/agent-linux).

**Extensions disabled**

Confirm that `osProfile.allowExtensionOperations` is set to `true` on the VM.

You can check this setting using the Azure CLI:

```bash
az vm show \
  --resource-group <resource-group> \
  --name <vm-name> \
  --query "osProfile.allowExtensionOperations"

```

If a policy disables extension operations, Run Command will not work.
