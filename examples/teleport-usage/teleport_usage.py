"""
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
"""

import json
import datetime
import boto3
import os

from boto3.dynamodb.conditions import Key, Attr


# Check for AWS DynamoDB table name and region ENV, if none prompt
table_name = os.getenv("TABLE_NAME")
if not table_name:
    TABLE_NAME = input("DynamoDB events table name: ")

aws_region = os.getenv("AWS_REGION")
if not aws_region:
    aws_region = input("AWS Region for DynamoDB: ")

# Check ENV for start time, end time is always 30 days later.
start_date = os.getenv("START_DATE")
if not start_date:
    now = datetime.datetime.now()
    start_date = now - datetime.timedelta(days=30)
else:
    start_date = datetime.datetime.strptime(start_date,'%Y-%m-%d')

end_date = start_date + datetime.timedelta(days=30)

# Configure DynamoDB for query
dynamodb = boto3.resource("dynamodb", region_name=aws_region)
table = dynamodb.Table(TABLE_NAME)

# variables for protocols
app = 'app.session.start'
db = 'db.session.start'
kube = 'kube.request'
ssh = 'session.start'
win = 'windows.desktop.session.start'

protocols = ['app', 'db', 'kube', 'win']
all_users = []

# Generating data message
start_date_str = start_date.strftime("%b %d %y")
end_date_str = end_date.strftime("%b %d %y")
day_range = (end_date - start_date).days
d_range = (end_date - start_date).days + 1
print("""
NOTE: This tool queries usage for the last 30 days")
For a custom start date use ENV START_DATE (format YYYY-MM-DD)")
""")
print(f"Gathering data for unique users who accessed each protocol between {start_date_str} to {end_date_str} ({day_range} days): ")

unique_users = set()

# Run Dynamodb query to retrieve all login for SSH for specified date range
for i in range(d_range):
    current_date = start_date + datetime.timedelta(days=i)

    date = current_date.strftime("%Y-%m-%d")
    response = table.query(
        TableName=TABLE_NAME,
        IndexName="timesearchV2",
        KeyConditionExpression=Key("CreatedAtDate").eq(date),
        FilterExpression=Attr("EventType").begins_with(ssh),
    )
    # Calculate unique users
    for item in response["Items"]:
        user = item.get("FieldsMap").get("user")
        if user:
            unique_users.add(user)
print(f"ssh: {len(unique_users)}")
if len(unique_users) != 0:
    all_users.append(f"ssh: {unique_users}")

# Loop through the protocols in the list
for protocol in protocols:
  # Create a set for the unique users of this protocol
  unique_users = set()

  # Run Dynamodb query to retrieve all login event types for specified date range
    for i in range(d_range):
        current_date = start_date + datetime.timedelta(i)
        date = current_date.strftime("%Y-%m-%d")
        response = table.query(
            TableName=TABLE_NAME,
            IndexName="timesearchV2",
            KeyConditionExpression=Key("CreatedAtDate").eq(date),
            FilterExpression=Attr("EventType").begins_with(protocol),
        )
        # Calculate unique users
        for item in response["Items"]:
            user = item.get("FieldsMap").get("user")
            if user:
                unique_users.add(user)

    # Print the count of unique users for this protocol
    print(f"{protocol}: {len(unique_users)}")
    if len(unique_users) != 0:
        all_users.append(f"{protocol}: {unique_users}")

# By default will not print all users, to print use ENV  SHOW_USERS=YES
print_users = os.getenv("SHOW_USERS")
if print_users == "YES":
    # Print the unique users, separated by a comma and a space
    print("Printing all unique users per protocol: ")
    print(json.dumps(all_users, indent=4, separators=(',', ':')))
else:
    print("If you would like to print all unique users per protocol set ENV SHOW_USERS=YES in a future run")
