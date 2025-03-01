# IAM Auth disabled
The Teleport Database Service uses [IAM authentication](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html) to communicate with RDS.

The following RDS databases do not have IAM authentication enabled.

You can enable by modifying the IAM DB Authentication property of the database.