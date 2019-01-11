## Terraform based provisioning examples (Amazon single AMI)

```bash
# export variables
export TF_VAR_ami_name="gravitational-teleport-ami-oss-3.0.1"
export TF_VAR_cluster_name=example.com
export TF_VAR_email="support@example.com"
export TF_VAR_key_name="ops"
export TF_VAR_license_path="/path/to/license"
export TF_VAR_region="us-west-2"
export TF_VAR_route53_domain="teleport.example.com"
export TF_VAR_route53_zone="example.com"
export TF_VAR_s3_bucket_name="teleport.example.com"

# plan
make plan
```

## Upgrade examples

All examples are run from `ansible` directory and are to illustrate
upgrade order of the provisioned infrastructure.

**Install python deps**

```
pip install boto3==1.0.0 ansible==2.7.0.0
```

**Configure AWS**

Make sure to configure [your aws creds](https://boto3.readthedocs.io/en/latest/guide/quickstart.html#configuration).

**Generate SSH config**

```
# generate SSH config for ansible to go through bastion
# this will write bastion
python ec2.py --ssh --ssh-key=/path/to/key
# make sure ansible works by pinging the nodes
ansible -vvv -i ec2.py -u admin auth -m ping --private-key=/path/to/key
```


**Launch an upgrade**

```
ansible-playbook -vvv -i ec2.py --private-key=/path/to/key --extra-vars "teleport_version=3.1.1" upgrade.yaml
```
