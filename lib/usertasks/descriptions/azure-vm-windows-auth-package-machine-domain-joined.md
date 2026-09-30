# VM is domain-joined

This VM is joined to an Active Directory domain. Azure Windows VM discovery only
supports non-domain-joined VMs using local accounts.

If this VM should be part of your Teleport cluster, use Teleport's Active
Directory-integrated Windows desktop discovery instead. Otherwise, exclude this
VM from the discovery config's Azure matcher, for example using a tag filter, to
stop it being retried.
