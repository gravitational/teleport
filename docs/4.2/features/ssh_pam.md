# Using Teleport with Pluggable Authentication Modules (PAM)

Teleport SSH daemon can be configured to integrate with [PAM](https://en.wikipedia.org/wiki/Linux_PAM). T
This allows Teleport to create user sessions using PAM session profiles.

Teleport only supports the `account` and `session` stack. The `auth` PAM module is
currently not supported with Teleport.


## Introduction to  Pluggable Authentication Modules

Pluggable Authentication Modules (PAM) was started in ..X.. So it could Y. 

```bash
$ man pam
```
The Pluggable Authentication Modules (PAM) library abstracts a number of common 
authentication-related operations and provides a framework for dynamically loaded 
modules that implement these operations in various ways.

Terminology
In PAM parlance, the application that uses PAM to authenticate a user is the server, 
and is identified for configuration purposes by a service name, which is often (but 
not necessarily) the program name.

The user requesting authentication is called the applicant, while the user (usually, root)
charged with verifying his identity and granting him the requested credentials is 
called the arbitrator.

The sequence of operations the server goes through to authenticate a user and perform
whatever task he requested is a PAM transaction; the context within which the server 
performs the requested task is called a session.

The functionality embodied by PAM is divided into six primitives grouped into four 
facilities: authentication, account management, session management and password 
management.

Teleport currently supports account management and session management.

## Setting up PAM on a Linux Machine running Teleport. 

To enable PAM on a given Linux machine, update `/etc/teleport.yaml` with:

```yaml
teleport:
   ssh_service:
      pam:
         # "no" by default
         enabled: yes
         # use /etc/pam.d/sshd configuration (the default)
         service_name: "sshd"
```

Please note that most Linux distributions come with a number of PAM services in
`/etc/pam.d` and Teleport will try to use `sshd` by default, which will be
removed if you uninstall `openssh-server` package. We recommend creating your
own PAM service file like `/etc/pam.d/teleport` and specifying it as
`service_name` above.


## Setting Message of the Day (motd) with Teleport

The file `/etc/motd` is normally displayed by login(1) after a user has logged in 
but before the  shell is run.  It is generally used for important system-wide announcements.

This feature ca
...

## Creating local users to Teleport

Teleport 4.3 introduced the ability to create local (UNIX) users on login. This is
very helpful if you're a large organization and want to create local user directors on
the fly. 


Description
Added ability to read in PAM environment variables from PAM handle and pass environment
variables to PAM module TELEPORT_USERNAME, TELEPORT_LOGIN, and TELEPORT_ROLES.

Refactored launching of shell to call PAM first. This allows a PAM module to create 
the user and home directory before attempting to launch a shell for said user.

To do this the command passed to Teleport during re-exec has changed. Before the 
Teleport master process would resolve the user fully (UID, GUID, supplementary groups,
shell, home directory) before re-launching itself to then launch a shell. However,
if PAM is used to create the user on the fly and PAM has not been called yet, 
this will fail.

Instead that work has now been pushed to occur in the child process. This means the
Teleport master process now creates a payload with the minimum needed from *srv.ServerContext
and will then re-exec itself. The child process will call PAM and then attempt to 
resolve the user (UID, GUID, supplementary groups, shell, home directory).

Examples
Using pam_exec.so
Using pam_exec.so is the easiest way to use the PAM stack to create a user if the
user does not already exist. The advantage of using pam_exec.so is that it usually
ships with the operating system. The downside is that it doesn't provide access to
 some additional environment variables that Teleport sets (see the pam_script.so 
 example for those) to use additional identity metadata in the user creation process.

You can either add pam_exec.so to your existing PAM stack for your application or 
write a new one for Teleport. In this example we'll write a new one to simplify how
 to use pam_exec.so with Teleport.

Start by creating a file `/etc/pam.d/teleport` with the following contents.

```bash
account   required   pam_exec.so /etc/pam-exec.d/teleport_acct
session   required   pam_motd.so
```

Note the inclusion of pam_motd.so under the session facility. While pam_motd.so is
not required for user creation, Teleport requires a module set for both the account
and session facility to work.

Next create the script that will be run by pam_exec.so like below. This script will
check if the user passed in PAM_USER exists and if it does not, it will create it.
Any error from useradd will be written to /tmp/pam.error.

```bash
mkdir -p /etc/pam-exec.d
cat > /etc/pam-exec.d/teleport_acct <<EOF
#!/bin/sh
id -u "${PAM_USER}" &>/dev/null  || /sbin/useradd -m "${PAM_USER}" 2> /tmp/pam.error
exit 0
EOF
chmod +x /etc/pam-exec.d/teleport_acct
```

Next update /etc/teleport.yaml to call the above PAM stack by both enabling PAM 
and setting the service_name.

```yaml
ssh_service:
   pam:
     enabled: yes
     service_name: "teleport"
```

Now attempting to login as an existing user should result in the creation of the user 
and successful login.

Using `pam_script.so`
If more advanced functionality is needed pam_script.so is a good choice. It typically 
has to be installed from packages but richer scripts with more identity information 
from Teleport can be used during the process of user creation.

To start install `pam_script.so`. On Debian based systems this would be 
`apt-get install libpam-script` and on RHEL based systems yum install pam-script.

You can either add pam_script.so to your existing PAM stack for your application 
or write a new one for Teleport. In this example we'll write a new one to simplify 
how to use `pam_script.so` with Teleport.

Start by creating a file `/etc/pam.d/teleport` with the following contents.

```sh
account   required   pam_script.so
session   required   pam_motd.so
```

Note the inclusion of `pam_motd.so` under the session facility. While pam_motd.so 
is not required for user creation, Teleport requires a module set for both the account
and session facility to work.

Next create the script that will be run by pam_script.so like below. This script 
will check if the user passed in TELEPORT_LOGIN exists and if it does not, it will
create it. Any error from useradd will be written to `/tmp/pam.error`. Note the 
additional environment variables TELEPORT_USERNAME, TELEPORT_ROLES, and TELEPORT_LOGIN.
These can be used to write richer scripts that may change the system in other 
ways based off identity information.

```bash
mkdir -p /etc/pam-script.d
cat > /etc/pam-script.d/teleport_acct <<EOF
#!/bin/sh
COMMENT="User ${TELEPORT_USERNAME} with roles ${TELEPORT_ROLES} created by Teleport."
id -u "${TELEPORT_LOGIN}" &>/dev/null  || /sbin/useradd -m -c "${COMMENT}" "${TELEPORT_LOGIN}" 2> /tmp/pam.error
exit 0
EOF
chmod +x /etc/pam-script.d/teleport_acct
```

Next update `/etc/teleport.yaml` to call the above PAM stack by both enabling PAM and 
setting the service_name.

```yaml
ssh_service:
   pam:
     enabled: yes
     service_name: "teleport"
```

Now attempting to login as an existing user should result in the creation of the
user and successful login.


