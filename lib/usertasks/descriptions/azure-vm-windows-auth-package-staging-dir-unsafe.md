# Unsafe installer staging directory

The installer refused to run because its staging directory in `%TEMP%` is a
reparse point (a symlink or junction), which could redirect the installer to
write to an untrusted location.

This is a safety check, not a transient failure. Investigate why `%TEMP%` is
redirected on this VM. This can be a sign of tampering or an unusual system
configuration. Remove the reparse point before retrying enrollment.
