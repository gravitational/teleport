import { useState, useEffect } from 'react';

export function useClipboardReadWrite(
  canUseClipboardReadWrite: boolean
): ClipboardPermissionStatus {
  const [permission, setPermission] = useState<ClipboardPermissionStatus>({
    state: '',
  });

  const read = useClipboardRead(canUseClipboardReadWrite);
  const write = useClipboardWrite(canUseClipboardReadWrite);

  useEffect(() => {
    if (read.state === 'error') {
      setPermission(read);
    } else if (write.state === 'error') {
      setPermission(write);
    } else if (read.state === 'denied' || write.state === 'denied') {
      setPermission({ state: 'denied' });
    } else if (read.state === 'prompt' || write.state === 'prompt') {
      setPermission({ state: 'prompt' });
    } else if (read.state === 'granted' && write.state === 'granted') {
      setPermission({ state: 'granted' });
    } else {
      setPermission({ state: '' });
    }
  }, [read, write, canUseClipboardReadWrite]);

  return permission;
}

function useClipboardRead(canUseClipboardReadWrite: boolean) {
  return useClipboardPermission(
    ClipboardPermissionType.Read,
    canUseClipboardReadWrite
  );
}

function useClipboardWrite(canUseClipboardReadWrite: boolean) {
  return useClipboardPermission(
    ClipboardPermissionType.Write,
    canUseClipboardReadWrite
  );
}

// If canUseClipboardReadWrite is set to false, then {state: ''} will simply be returned.
// This is desireable so that useClipboardPermission can always be used unconditionally as a hook,
// even in cases where we don't want to check/prompt-for the premission i.e. if the user isn't using
// a Chromium based browser.
function useClipboardPermission(
  readOrWrite: ClipboardPermissionType,
  canUseClipboardReadWrite: boolean
) {
  const permissionName = readOrWrite as unknown as PermissionName;

  const [permission, setPermission] = useState<ClipboardPermissionStatus>({
    state: '',
  });

  const setPermissionOrPromptUser = () => {
    navigator.permissions.query({ name: permissionName }).then(perm => {
      if (perm.state === 'granted' || perm.state === 'denied') {
        if (permission.state !== perm.state) {
          setPermission({ state: perm.state });
        }
      } else {
        // result.state === 'prompt';
        if (permission.state !== 'prompt') {
          setPermission({ state: 'prompt' });
          // return, because the setPermission above will trigger the useEffect below,
          // which will cause us to end up back in the parent else-clause but skip this if-clause.
          return;
        }
        // Force the prompt to appear for the user
        navigator.clipboard
          .readText()
          .then(() => {
            // readText's promise only resolves if permission is granted.
            setPermission({ state: 'granted' });
          })
          .catch(err => {
            if (err && err.name === 'NotAllowedError') {
              // NotAllowedError will be caught if the permission was 'denied' or remained 'prompt'.
              // Try again, which result in either setPermission('denied') or re-prompting the user.
              setPermissionOrPromptUser();
            } else {
              if (err && err.message) {
                setPermission({ state: 'error', errorText: err.message });
              } else {
                setPermission({
                  state: 'error',
                  errorText:
                    'unknown error reading browser clibpoard permissions',
                });
              }
            }
          });
      }
    });
  };

  useEffect(() => {
    if (canUseClipboardReadWrite) {
      setPermissionOrPromptUser();
    }
  }, [permission, canUseClipboardReadWrite]);

  return permission;
}

enum ClipboardPermissionType {
  Read = 'clipboard-read',
  Write = 'clipboard-write',
}

type ClipboardPermissionStatus = {
  state: PermissionState | 'error' | '';
  errorText?: string;
};
