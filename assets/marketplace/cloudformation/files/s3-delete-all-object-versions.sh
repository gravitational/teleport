#!/bin/bash
if [[ "${AWS_REGION}" == "" ]]; then
    echo "AWS_REGION must be set"
    exit 1
fi
bucket=$1
if [[ "${bucket}" == "" ]]; then
    echo "Usage: $(basename $0) <bucket>"
    exit 1
fi

set -e
echo "Removing all versions from ${bucket}"

versions=$(aws s3api list-object-versions --bucket ${bucket} | jq '.Versions')
markers=$(aws s3api list-object-versions --bucket ${bucket} | jq '.DeleteMarkers')
let count=$(echo ${versions} | jq 'length')-1

if [ ${count} -gt -1 ]; then
        echo "removing files"
        for i in $(seq 0 ${count}); do
                key=$(echo ${versions} | jq .[$i].Key | sed -e 's/\"//g')
                versionId=$(echo ${versions} | jq .[$i].VersionId | sed -e 's/\"//g')
                cmd="aws s3api delete-object --region=${AWS_REGION} --bucket ${bucket} --key ${key} --version-id ${versionId}"
                echo $cmd
                $cmd
        done
fi

let count=$(echo ${markers} | jq 'length')-1

if [ ${count} -gt -1 ]; then
        echo "removing delete markers"
        for i in $(seq 0 ${count}); do
                key=$(echo ${markers} | jq .[$i].Key | sed -e 's/\"//g')
                versionId=$(echo ${markers} | jq .[$i].VersionId | sed -e 's/\"//g')
                cmd="aws s3api delete-object --region=${AWS_REGION} --bucket ${bucket} --key ${key} --version-id ${versionId}"
                echo $cmd
                $cmd
        done
fi
