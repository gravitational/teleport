## Terraform based provisioning examples

```bash
# export vairables
export TF_VAR_region="us-west-2"
export TF_VAR_cluster_name=example.com
export TF_VAR_teleport_version="2.5.0-alpha.4"
export TF_VAR_key_name="ops"
export TF_VAR_license_path="/path/to/license"
export TF_VAR_ami_name="debian-stretch-hvm-x86_64-gp2-2018-01-06-16218-572488bb-fc09-4638-8628-e1e1d26436f4-ami-628ad918.4"
export TF_VAR_route53_zone="example.com"
export TF_VAR_route53_domain="teleport.example.com"
export TF_VAR_s3_bucket_name="teleport.example.com"
export TF_VAR_email="support@example.com"

# plan
make plan
```
