# Create Access List which allows, users of an Entra Group, access to discovered EKS Clusters

The Entra ID Teleport Access List Integration program allows users from a given Entra Group to access all auto discovered EKS Clusters for a given AWS account ID.

## Prequisites

- Go version 1.24.4 or higher
- Ensure you have the required Teleport persmissions.
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
        
- ```tsh``` installed locally
- Defined Entra Group's you plan to assign to Access Lists that grant access to EKS clusters in specificed AWS accounts.

## What does the intregation create?

- First, it creates a Teleport Role which allows access to auto discovered EKS Clusters for a given account id:
  
          yaml
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
      
***If a role named `eks-access` already exists, this step is skipped.***

- Next, the program creates an Access List with the following properties:
    - The Entra Group is added as a member of the Access List, granting the Access List permissions to members of the Entra Group.
    - Permission setup includes access to the role above with a specific account id (eg 123456789012), which allows users access to all 
      Kubernetes resources with these labels:
        ```yaml
            kubernetes_labels:
              account-id: "123456789012"
              teleport.dev/discovery-type: eks
        ```
***You'll have a single Teleport Role, but as many Access Lists as Entra Groups.***

## How to run the Entra ID Access List program

  1. Copy the following files from this git directory to where you plan to run the program:
     - ```main.go```
     - ```go.mod```
     - ```go.sum```
  2. You must provide three properties to this tool:
    - `-proxy` flag or `TELEPORT_PROXY` env var: this is your cluster endpoint address (eg. `example.teleport.sh:443`)
    - `-aws-account-id` flag or `AWS_ACCOUNT_ID` env var: this is the account id that will be used to create the access list (eg. `123456789012`)
    - the entra group, which can be identified using any one of the following:
      - `-group-by-teleport-id` flag or `GROUP_BY_TELEPORT_ID` 
         - env var: this is the id of the Entra Group as seen in Teleport
      - `-group-by-entra-object-id` flag or `GROUP_BY_ENTRA_OBJECT_ID` 
         - env var: this is the Entra Group Object ID as seen in Microsoft Entra dashboard
      - `-group-by-name` flag or `GROUP_BY_ENTRA_NAME` 
        - env var: this is the Entra Group name (display name/title) as seen in Microsoft Entra dashboard
 3. Replace the properities in the following example with the values you gathered in the previous step and run the program.
    ***Ensure you have valid Teleport credentials (eg, tsh login) before running this command:***
    
              shell
              $ go run main.go -aws-account-id 123456789012 -group-by-name MarcoNVTest -proxy example.teleport.sh
              2025/06/20 16:52:51 INFO Using existing Teleport Role. role_name=eks-access
              2025/06/20 16:52:52 INFO Using Access List access_list_name=eks-access-123456789012 access_list_title="EKS Access for 123456789012" account_id=123456789012
              2025/06/20 16:52:52 INFO Entra Group ID found teleport_access_list_name=3ab57a50-8c5c-5e15-9df1-82e3e443ad57 entra_group_name=MarcoNVTest entra_group_object_id=""
              2025/06/20 16:52:52 INFO Entra Group ID added as a group member access_list=eks-access-123456789012 member_group_name=3ab57a50-8c5c-5e15-9df1-82e3e443ad57
            

5. Verify that the Access List has been created with the Entra Group assigments you defined in the previous step.

### Troubleshooting

Full list of arguments:

 - ```-aws-account-id```
    - string 
    - AWS Account ID to allow access to (required).
 - ```-group-by-entra-object-id```
    - string
    - Microsoft Entra Group Object ID.
 - ```-group-by-name```
    - string
    - Teleport Teleport Entra Group Access List ID as synced into Teleport
 - ```-group-by-teleport-id```
    - string
    - Teleport's ID for the Access List.
 - ```-proxy```
    - string
    - Teleport Proxy's address, eg. tenant.teleport.sh:443 (required).
