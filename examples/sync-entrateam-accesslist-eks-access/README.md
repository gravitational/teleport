# Create Access List which allows, users of an Entra Group, access to discovered EKS Clusters

This program allows users from a given Entra Group to access all auto discovered EKS Clusters for a given AWS account ID.

It does the following:
 - create a Teleport Role which allows access to auto discovered EKS Clusters for a given account ID, which is inherited from a trait
 - create an Access List per account ID that:
   - members get access to the role above
   - injects the AWS account ID as a trait
   - adds another Access List as a sub-list/member, which must be a Entra Group

You'll have a single Teleport Role, but as many Access Lists as Entra Groups.

You can provide the Entra ID Group using one of:
- the Teleport's Access List ID as seen in Teleport
- the Entra Group Object ID as seen in Azure/Entra
- the Entra Group Name

Ensure you have valid Teleport credentials (eg, tsh login) before running this command.
The following Teleport RBAC rules are required:
```yaml
    - resources:
      - roles
      verbs:
      - read
      - list
      - create
    - resources:
      - kube_server
      verbs:
      - read
      - list
    - resources:
      - access_list
      verbs:
      - read
      - list
      - create
      - update
```

You must run this command for each EKS Cluster / AWS Account ID / Entra Group.

Example:
```shell
$ go run main.go -aws-account-id 123456789012 -group-by-name=MarcoNVTest
2025/06/20 16:52:51 INFO Using existing Teleport Role. role_name=eks-access
2025/06/20 16:52:52 INFO Found Kubernetes cluster account_id=123456789012 cluster_name=MarcoNV03-eks-eu-south-2-123456789012
2025/06/20 16:52:52 INFO Using Access List access_list_name=eks-access-123456789012 access_list_title="EKS Access for 123456789012" account_id=123456789012
2025/06/20 16:52:52 INFO Entra Group ID found teleport_access_list_name=3ab57a50-8c5c-5e15-9df1-82e3e443ad57 entra_group_name=MarcoNVTest entra_group_object_id=""
2025/06/20 16:52:52 INFO Entra Group ID added as a group member access_list=eks-access-123456789012 member_group_name=3ab57a50-8c5c-5e15-9df1-82e3e443ad57
```