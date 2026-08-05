# Installer checksum mismatch

The downloaded Teleport Windows Authentication Package installer did not match
its expected checksum, so it was not run.

This is usually caused by a corrupted download and will often succeed on retry.
If the problem persists across multiple discovery cycles, check whether a proxy
or network appliance between the VM and the Teleport cluster is modifying
traffic in transit.
