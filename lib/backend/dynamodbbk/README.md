DynamoDB backend implementation for Teleport.

WARNING: it will involve reccuring charge from AWS.
The table created by the backend will provision 5/5 R/W capacity.
It should be covered by the free tier.

How to use it ?
Install the auth server and add this storage configuration in teleport section:
  storage:
    type: dynamodb
    region: eu-west-1
    table_name: prod.teleport.auth
    access_key: XXXXXXXXXXXXXXXXXXXXX
    secret_key: YYYYYYYYYYYYYYYYYYYYY

replace region and table_name with appropriate settings.

You can use IAM role instead of hard coded access and secret key (IAM role is recommended).
You must apply correct policy in order to the auth to create/get/update K/V in DynamoDB.

Example of a typical policy:

{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllAPIActionsOnTeleportAuth",
            "Effect": "Allow",
            "Action": "dynamodb:*",
            "Resource": "arn:aws:dynamodb:us-west-2:123456789012:table/prod.teleport.auth"
        }
    ]
}
