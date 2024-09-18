#!/bin/bash

# Clean up
rm create_iam_user.zip
rm -rf package

# Download dependencies and make the zip
mkdir package
pip install --target ./package pymongo
cd package
zip -r ../create_iam_user.zip .
cd -
zip create_iam_user.zip create_iam_user.py

# Clean up
rm -rf package
