# Enrollment failed
Teleport could not auto enroll the Azure VM due to an unexpected error.
This issue is used when the failure does not match a known Azure error pattern.

To troubleshoot, check the following:

**Teleport Discovery Service logs**

Look for Azure API errors or Run Command failures around the enrollment time.
The logs usually include the underlying Azure error message.

**Azure Activity Log**

Check the VM activity log for failed Run Command or extension operations.
This helps identify permission or policy problems at the Azure layer.
