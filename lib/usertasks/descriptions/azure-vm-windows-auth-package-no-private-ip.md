# No private IP address found

Teleport could not resolve a private IP address for this VM from its network interfaces.
A private IP address is required so the Windows Desktop Service can connect to the desktop.

This usually means one of the following:
- The VM has no network interface attached.
- None of the VM's network interfaces have a private IP address configured.
- The VM was recently created and Azure has not finished provisioning its networking yet.

Check the VM's network interfaces in the Azure Portal and confirm at least one has a private IP
address assigned. If the VM was just created, this will often resolve itself on the next
discovery cycle.
