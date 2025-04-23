---
title: Teleport Authentication with Google Workspace (G Suite) and SAML
description: How to configure Teleport access with SAML in Google Workspace
---

This guide will describe how to set up Teleport to use SAML to authenticate against Google Workspace. This has some advantages over OIDC, Namely that you can explicitly control what attributes are passed to Teleport (groups, traits etc).

**Teleport Configuration and presumptions:**

- You are a super administrator of a Google Workspace Instance
- You are running a v17.x version of Teleport
- Local authentication for Teleport is functional
- You are a Teleport administrator (member of editor role)

### Setup SAML Application on Google:
**Step 1**
[Sign in](https://admin.google.com/) with a super administrator account to the Google Admin console.
Go to Menu and then [Apps > Web and mobile apps](https://admin.google.com/ac/apps/unified).
Click Add App and then Add custom SAML app.
Provide a name and description and optional [icon](https://goteleport.com/api/files/teleport-logo-package-07-17-24.zip) I used the white square resized to 256x256 pixels

<img width="1448" alt="Screenshot 2025-04-15 at 1 06 17 PM" src="https://github.com/user-attachments/assets/a29c7325-d181-4961-a725-5da0e6894819" />

**Step 2:**
Download the Google IdP Metadata, you will need this later on.
<img width="1442" alt="Screenshot 2025-04-15 at 1 06 30 PM" src="https://github.com/user-attachments/assets/aa5c2dfc-41db-4d99-8ab2-49516955a056" />

**Step 3:**
Fill in the Service Provider (Teleport) details:
<img width="1428" alt="Screenshot 2025-04-15 at 1 09 33 PM" src="https://github.com/user-attachments/assets/cd65fc1c-fb59-4ce0-a7d9-91a47dff5aa0" />

- ACS URL— https:// < cluster-url >:< port >/v1/webapi/saml/acs/google-saml
- Entity ID— https:// < cluster-url >:< port >/v1/webapi/saml/acs/google-saml
- Start URL— https:// < cluster-url >(e.g. https://tenantname.teleport.sh)
- Click signed response on

For Name ID

- Choose Name ID Format EMAIL
- for Name ID, the default is what we want. "Basic Information -> Primary Email"

**Step 4:**
Set up the mapping between Google and Teleport, this is where we set what is sent to Teleport for a users login and to set up role mapping based on a users groups. 

Click Add Mapping
under Basic Information choose Primary Email, for app attributes (this is what we will use in Teleport) enter "username"

For Group membership
On the left side you can choose any groups on your Google instance, for app attribute (what we will use in Teleport) enter "groups"
<img width="1452" alt="Screenshot 2025-04-15 at 1 13 31 PM" src="https://github.com/user-attachments/assets/2487ea53-fc6c-4175-b267-58b2f05370df" />

Click Finish, We have a working SAML instance on the Google side, however it is disabled for our users. 
To enable it (you can do this now or come back to it after we set up the Teleport side)
Click on the application:
<img width="898" alt="Screenshot 2025-04-15 at 1 35 19 PM" src="https://github.com/user-attachments/assets/f7a7db95-a978-4f2c-9c64-e5665e682877" />
At the top where it says User Access, click anywhere there and choose on. (It may take a minute to enable)
<img width="1446" alt="Screenshot 2025-04-15 at 1 36 24 PM" src="https://github.com/user-attachments/assets/1108a0f7-d25a-4733-a598-d9c904e2dc9d" />

We are half way done!

### Set up the Auth connector on Teleport:
**Step 1**
Log into Teleport UI as a user with the editor role. 
On the left side choose Zero Trust Access -> Auth Connectors

<img width="370" alt="Screenshot 2025-04-15 at 1 39 37 PM" src="https://github.com/user-attachments/assets/57fb00ca-c079-49e6-8b8f-d32a4be27785" />

Click Add Auth Connector (top right)
<img width="404" alt="Screenshot 2025-04-15 at 1 41 45 PM" src="https://github.com/user-attachments/assets/f0357b8f-4c50-4251-b4bc-8e5f13fa54ff" />

Choose SAML Connector
<img width="450" alt="Screenshot 2025-04-15 at 1 42 47 PM" src="https://github.com/user-attachments/assets/5941e163-9e3e-46f6-8684-3d4b0456e0ef" />

**Step 2**
The YAML editor will appear.

> (Optional) you can permit IdP-initiated SSO by adding spec.allow_idp_initiated: true There are important security considerations to be aware of though. This feature is potentially unsafe and should be used with caution.
> Enabling IdP-initiated login comes with notable security risks such as:
> - Possibility of replay attacks on the SAML payload giving an attacker a secret web session
> - Increased risk of session hijacking and impersonation attacks based on intercepting SAML communications
- For line 9 you will want to give this connector a name (Google SAML or anything you may like)
- For line 13 you will want to provide a text string that is presented to your users when then log into Teleport (Google Login or anything else you may like)
- For line 17 you will want to provide the *exact* URL we used in the beginning (https:// < cluster-url >:< port >/v1/webapi/saml/acs/google-saml)
- For lines 19 -20 for value you will want to put the name of a group you mapped in Step 4 to a role in Teleport. This is required for at least one role otherwise the user will not have a role and be unable to login. You can remove the unused mappings. Value is the exact name of the group on the Google side, role is the corresponding role on the Teleport side. 
- For lines 26 to the end you will want the open the IdP metadata file in a text editor that you downloaded in Step 2.
- The indenting is critical that its preserved. The beginning of the metadata you obtained from Google should line up under the "t" in entity. 
<img width="1165" alt="Screenshot 2025-04-15 at 1 44 02 PM" src="https://github.com/user-attachments/assets/c56e2a49-62b8-42d2-8037-95c543334ed2" />

Click Save Changes

**Step 3**
Create or update a role that this user will get. These should be mapped in the step above. In my test instance I mapped a group called "Teleport Developers" from Google to a role called "teleport-developers" in Teleport. You can also update the logins portion in an existing role. Everything else in the role is your discretion, however for logins you may want to do the following. In the example below the users username will be picked up from Google. Teleport will create an attribute for logins with the same thing minus the email domain. (eg paul@mydomain.com becomes paul)
```
    logins:
    - '{{email.local(external.logins)}}'
```

At this point we can test! 
**IMPORTANT Note:** if you didn't enable the SAML application at the end of Step 4 in the first half you will need to revisit that portion and enable it. 
In an incognito window go to your tenant URL. You should have an additional login button. Click it, and you will get redirected to a Google login prompt. 
<img width="636" alt="Screenshot 2025-04-15 at 1 57 09 PM" src="https://github.com/user-attachments/assets/658a33e3-fe50-4bbb-a53c-b38195d15a4c" />

Also, since we added this as a SAML application a user in your Google Workspace can also access Teleport from the Google UI
<img width="324" alt="Screenshot 2025-04-15 at 1 59 59 PM" src="https://github.com/user-attachments/assets/9c1bd2d0-d139-4e7a-8973-614347eafec9" /> 
_IMPORTANT: This will only work if you permitted IdP-initiated SSO in Step 2 of the Teleport setup portion of this document._ 

Once a user has logged in once you can use tctl and examine this user and what attributes are sent from Google. (note I did notice it took 30-50 seconds from updating a trait to map from the Google side to when I started seeing it through Teleport)
example:
```
% tctl get user/paul@mydomain.com
kind: user
metadata:
  expires: "2025-04-17T01:42:37.191242927Z"
  name: paul@mydomain.com
  revision: 7850bd9d-1a5e-4ee0-984c-a3b9ca6c4ea7
spec:
  created_by:
    connector:
      id: google-saml
      identity: paul@mydomain.com
      type: saml
    time: "2025-04-15T19:42:37.191246446Z"
    user:
      name: system
  expires: "0001-01-01T00:00:00Z"
  roles:
  - teleport-developers

  saml_identities:
  - connector_id: google-saml
    username: paul@mydomain.com
  status:
    is_locked: false
    lock_expires: "0001-01-01T00:00:00Z"
    locked_time: "0001-01-01T00:00:00Z"
  traits:
    groups:
    - teleport-developers
    logins: null
    username:
    - paul@mydomain.com
```


