#!/usr/bin/env python

'''
EC2 external inventory script
=================================

Generates inventory that Ansible can understand by making API request to
AWS EC2 using the Boto library.

NOTE: This script assumes Ansible is being executed where the environment
variables needed for Boto have already been set:

https://boto3.readthedocs.io/en/latest/guide/quickstart.html#configuration
'''

import boto3
import argparse
import os


ec2 = boto3.client('ec2')

parser = argparse.ArgumentParser(description='Generate ansible.cfg and dynamic ansible inventory')
parser.add_argument('--list', dest='list', help='produce dynamic ansible inventory when called with a list', action='store_true')
parser.add_argument('--ssh', dest='ssh', help='generate ssh config to use bastion', action='store_true')
parser.add_argument('--ssh-key', dest='ssh_key', help='key to use for ansible', default=None)
parser.add_argument('--cluster', dest='cluster', help='specify default cluster name via env varialbe TF_VAR_cluster_name', default=os.environ.get('TF_VAR_cluster_name', None))

args = parser.parse_args()

def generate_inventory(cluster):
    env = {
        'auth': [],
        'proxy': [],
        'node': [],
    }
    for role in env:
        response = ec2.describe_instances(
            Filters=[
            {
                'Name': 'tag:TeleportCluster',
                'Values': [
                    cluster,
                ]
            },
            {
                'Name': 'tag:TeleportRole',
                'Values': [
                    role,
                ]
            },
        ]
        )
        for reservation in response['Reservations']:
            for instance in reservation['Instances']:
                env[role].append(instance['PrivateDnsName'])
    return env

ssh_cfg_template = '''# {bastion} is a bastion host for ansible
Host {bastion}
    HostName {bastion}
    Port 22
    {identity_file}

# connect to nodes in the local cluster using bastion
Host *.compute.internal
    HostName %h
    Port 22
    ProxyCommand ssh {ssh_key_path} -p 22 %r@{bastion} nc %h %p
'''

def generate_ssh_cfg(cluster, ssh_key=None):
    ssh_key_path = ''
    identity_file = ''
    if ssh_key is not None:
        ssh_key_path = '-i {path}'.format(path=ssh_key)
        identity_file = 'IdentityFile {path}'.format(path=ssh_key)

    response = ec2.describe_instances(
        Filters=[
            {
                'Name': 'tag:TeleportCluster',
                'Values': [
                    cluster,
                ]
            },
            {
                'Name': 'tag:TeleportRole',
                'Values': [
                    'bastion',
                ]
            },
        ]
    )
    bastion = ''
    for reservation in response['Reservations']:
        for instance in reservation['Instances']:
            bastion = instance['PublicIpAddress']
    return ssh_cfg_template.format(bastion=bastion, ssh_key_path=ssh_key_path, identity_file=identity_file)

if args.cluster is None:
    print "env.py: error: provide --cluster either explicitly or by setting TF_VAR_cluster_name variable"
    exit(255)

if args.list:
    print generate_inventory(args.cluster)
elif args.ssh:
    with open('ssh.config', 'w') as f:
        out =  generate_ssh_cfg(args.cluster, args.ssh_key)
        f.write(out)
    print "generated file ./ssh.config"








