## DynamoDB examples

Examples require terraform installed.

### Autoscaling DynamoDB

* [Autoscaling](autoscale) - examples of setting up autoscaling for DynamoDB Teleport backend table

Plan the terraform changes:

```bash
REGION=us-west-2 TABLE_NAME=teleport.example.table make autoscale-plan
```

Set up autoscaling:

```bash
REGION=us-west-2 TABLE_NAME=teleport.example.table make autoscale-apply
```

Turn off autoscaling

```bash
REGION=us-west-2 TABLE_NAME=teleport.example.table make autoscale-apply
```


