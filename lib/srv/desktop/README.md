# Windows Desktop Access Beta

## How to set up Desktop Access on Windows

### Install Windows Server 2012

Download and instal a trial version of Windows Server 2012 R2 from:
https://www.microsoft.com/en-us/evalcenter/evaluate-windows-server-2012-r2

Windows Server 2012 is the oldest version we support.

See [this appendix](#appendix-virtualbox-notes) if using VirtualBox.

### Set up Active Directory

#### AD DS

First, we need to install Active Directory Domain Services (AD DS). From Server
Manager, in the top-right select `Manage > Add Roles and Features`.

In the wizard, select:
- Before you Begin: click `Next`
- Installation Type: `Role-based or feature-based installation`
- Server Selection: click `Next` (should be only one server - current one)
- Server Roles: select `Active Directory Domain Services`, in the popup click `Add Features`
- Features: click `Next`
- AD DS: click `Next`
- Confirmation: click `Install`, wait for completion
- Results: click `Promote this server to a domain controller` blue link

Another wizard (AD DS configuration) will open:
- Deployment Configuration: select `Add a new forest`, type in the Root domain
  name. Any DNS-like name will work, for example `example.com`, **write down
  this name for later**. Click `Next`.
- Domain Controller Options: type in a `Directory Services Restore Mode (DSRM)
  password` and click `Next`.
- DNS Options: click `Next`
- Additional Options: wait for the NetBIOS name to be generated, **write it
  down for later**, click `Next.`
- Paths: click `Next`.
- Review Options: click `Next`.
- Prerequisites Check: wait for the check to pass and click `Install`.
- Results: after ~10s, VM will restart

#### AD CS

Next, we'll install Active Directory Certificate Services (AD CS) to enable TLS
on LDAP connections. While AD CS is not strictly required, it is the easiest way
to generate a keypair and ensure that the server supports LDAPS. From Server
Manager, in the top-right select `Manage > Add Roles and Features`. (Note: you
won't be able to install both AD DS and AD CS at the same time, they need to be
separate).

In the wizard, select:
- Before you Begin: click `Next`
- Installation Type: `Role-based or feature-based installation`
- Server Selection: click `Next` (should be only one server - current one)
- Server Roles: select `Active Directory Certificate Services`, in the popup click `Add Features`
- Features: click `Next`
- AD CS: click `Next`
- Role Services: select `Certification Authority` and click `Next`
- Confirmation: do not select `Restart the destination server automatically if
  required`, click `Install`, wait for completion
- Results: click `Configure Active Directory Certificate Services on the
  destination server` blue link

Another wizard (AD CS configuration) will open:
- Credentials: click `Next`.
- Role Services: select `Certification Authority`, click `Next`.
- Setup Type: select `Enterprise CA`, click `Next`.
- CA Type: select `Root CA`, click `Next`.
- Private Key: select `Create a new private key`, click `Next`.
- Cryptography: click `Next`.
- CA Name: click `Next`.
- Validity Period: click `Next`.
- Certificate Database: click `Next`.
- Confirmation: click `Configure`.
- Results: click `Close`.

Restart the VM after configuring AD CS.

### Start Teleport

If this is an existing cluster, you need to rotate the CA (to enable CRL
creation required by Windows). Run `tctl auth rotate --grace-period=1m`.

Add the following to `teleport.yaml`:

```yaml
windows_desktop_service:
  enabled: yes
  listen_addr: "localhost:3028"
  ldap:
    addr:     "VM_IP:389"
    domain:   "DOMAIN_NAME"
    username: 'NETBIOS_DOMAIN_NAME\Administrator'
    password: 'PASSWORD'
  hosts:
  - "VM_IP"
```

Where:
- `VM_IP` is the IP address of your Windows VM (run `ipconfig.exe` from
  PowerShell to get this)
- `DOMAIN_NAME` is the AD domain name
- `NETBIOS_DOMAIN_NAME` is the NetBIOS domain name
- `PASSWORD` is the password for the `Administrator` user on the VM

Start `teleport`.

### Update AD group policy

On the VM, open "Start" menu and run "Group Policy Management". On the left
pane, select `your forest > Domains > your domain`. Right click on "Default
Domain Policy" and select "Edit...".

#### Enable remote RDP connections

In the group policy editor, select:

```
Computer Configuration > Policies > Administrative Templates > Windows Components > Remote Desktop Services > Remote Desktop Session Host > Connections
```

Right click on `Allow users to connect remotely by using Remote Desktop
Services` and select "Edit". Select "Enable" and "OK".

Under:

```
Computer Configuration > Policies > Administrative Templates > Windows Components > Remote Desktop Services > Remote Desktop Session Host > Security
```

Right click `Require user authentication for remote connections by using
Network Level Authentication`, edit, select **"Disable"** and "OK".

#### Install Teleport CA into Group Policy

Get the Teleport CA cert using `tctl auth export --type=windows >user-ca.cer`.
Transfer the `user-ca.cer` file to your VM (e.g. using a shared folder in
VirtualBox).

In the group policy editor, select:

```
Computer Configuration > Policies > Windows Settings > Security Settings > Public Key
Policies
```

Right click on `Trusted Root Certification Authorities` and select `Import`.
Click through the wizard, selecting your CA file.

To make sure that all Group Policy settings have synced, run `gpupdate.exe
/force` from PowerShell.

#### Enable Smart Card service

In the group policy editor, select:

```
Computer Configuration > Policies > Windows Settings > Security Settings > System Services
```

Double click on `Smart Card`, select `Define this policy setting` and switch to
`Automatic`. Click "OK".

#### Open RDP port in firewall

In the group policy editor, select:

```
Computer Configuration > Policies > Windows Settings > Security Settings > Windows Firewall and Advanced Security
```

Right click on `Inbound Rules` and select `New Rule...`. Under `Predefined`
select `Remote Desktop`. Only select the rule for `User Mode (TCP-in)`. On the
next screen, select `Allow the connection` and finish.

**Warning**: sometimes, firewall rules mysteriously disappear from the Group
Policy, I don't know why. If your desktop connections hang, check Group Policy
and re-add the rule if needed.

#### Sync group policy changes

After making changes, Windows should eventually sync local settings with the
group policy. If it's taking too long, you can force a sync by running
`gpupdate /force` from PowerShell.

### Try to log in

Go to the Teleport web UI. Under Desktops, click Connect next to your VM entry,
type in `Administrator` (or other existing account name) and hit Enter.

You should see the login screen for a few seconds, then it will auto-login you.

## Appendix: VirtualBox notes

Some advice to make the setup easier with VirtualBox.

First, [install VirtualBox Guest
Additions](https://www.virtualbox.org/manual/ch04.html).

Next, create a shared folder via VM settings in VirtualBox. This will let you
easily transfer files between host and VM.

Finally, switch the Network Adapter to Bridged Adapter mode. This lets you
connect to the VM from your host. To get the VM IP, run `ipconfig.exe` from
PowerShell.
