# Create Access List which allows, users of an Entra Group, access to discovered EKS Clusters

This program allows users from a given Entra Group to access all auto discovered EKS Clusters for a given AWS account ID.

First, it creates a Teleport Role which allows access to auto discovered EKS Clusters for a given account id:
```yaml
kind: role
metadata:
  name: eks-access
spec:
  allow:
    kubernetes_groups:
    - system:masters
    kubernetes_labels:
      account-id: '{{external["account-id"]}}'
      teleport.dev/discovery-type: eks
version: v7
```
If a role named `eks-access` already exists, this step is skipped.

Then, it creates an Access List which has the following properties:
- the Entra Group is added as a member, so all the members of the group have access to this access list permissions
- permission setup includes access to the role above with a specific account id (eg 123456789012), which allows users access to all Kubernetes resources with these labels:
```yaml
    kubernetes_labels:
      account-id: "123456789012"
      teleport.dev/discovery-type: eks
```

You'll have a single Teleport Role, but as many Access Lists as Entra Groups.

## Usage
You must provide 3 properties to this tool:
- `-proxy` flag or `TELEPORT_PROXY` env var: this is your cluster endpoint address (eg. `example.teleport.sh:443`)
- `-aws-account-id` flag or `AWS_ACCOUNT_ID` env var: this is the account id that will be used to create the access list (eg. `123456789012`)
- the entra group, which can be identified using any one of the following:
  - `-group-by-teleport-id` flag or `GROUP_BY_TELEPORT_ID` env var: this is the id of the Entra Group as seen in Teleport
  - `-group-by-entra-object-id` flag or `GROUP_BY_ENTRA_OBJECT_ID` env var: this is the Entra Group Object ID as seen in Microsoft Entra dashboard
  - `-group-by-name` flag or `GROUP_BY_ENTRA_NAME` env var: this is the Entra Group name (display name/title) as seen in Microsoft Entra dashboard

```bash
$ go run main.go -help
Usage of sync:
Create an Access List which allows users from an Entra ID Group to access EKS Clusters in a given AWS Account.

You can provide the Entra ID Group using one of:
- the Teleport's Access List ID as seen in Teleport
- the Entra Group Object ID as seen in Azure/Entra
- the Entra Group Name

Ensure you have valid Teleport credentials (eg, tsh login) before running this command.
The following Teleport RBAC rules are required:
    - resources:
      - roles
      verbs:
      - read
      - list
      - create
    - resources:
      - access_list
      verbs:
      - read
      - list
      - create
      - update

Full list of arguments:
  -aws-account-id string
        AWS Account ID to allow access to (required).
  -group-by-entra-object-id string
        Microsoft Entra Group Object ID.
  -group-by-name string
        Teleport Teleport Entra Group Access List ID as synced into Teleport
  -group-by-teleport-id string
        Teleport's ID for the Access List.
  -proxy string
        Teleport Proxy's address, eg. tenant.teleport.sh:443 (required).
```

Example:
```shell
$ go run main.go -aws-account-id 123456789012 -group-by-name MarcoNVTest -proxy example.teleport.sh
2025/06/20 16:52:51 INFO Using existing Teleport Role. role_name=eks-access
2025/06/20 16:52:52 INFO Using Access List access_list_name=eks-access-123456789012 access_list_title="EKS Access for 123456789012" account_id=123456789012
2025/06/20 16:52:52 INFO Entra Group ID found teleport_access_list_name=3ab57a50-8c5c-5e15-9df1-82e3e443ad57 entra_group_name=MarcoNVTest entra_group_object_id=""
2025/06/20 16:52:52 INFO Entra Group ID added as a group member access_list=eks-access-123456789012 member_group_name=3ab57a50-8c5c-5e15-9df1-82e3e443ad57
```