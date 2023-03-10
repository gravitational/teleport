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

import datetime
import boto3
import os

from boto3.dynamodb.conditions import Key, Attr


# Check for AWS DynamoDB table name and region ENV, if none prompt
table_name = os.getenv("TABLE_NAME")
if not table_name:
    table_name = input("DynamoDB events table name: ")

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
table = dynamodb.Table(table_name)

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

# Creating a set per protocol to gather unique users
unique_users = {
    'app.session.start': set(),
    'db.session.start': set(),
    'kube.request': set(),
    'session.start': set(),
    'windows.desktop.session.start': set(),
    
}

# Run Dynamodb query to retrieve all login for SSH for specified date range
for i in range(d_range):
    current_date = start_date + datetime.timedelta(days=i)

    date = current_date.strftime("%Y-%m-%d")
    response = table.query(
        TableName=table_name,
        IndexName="timesearchV2",
        KeyConditionExpression=Key("CreatedAtDate").eq(date),
        FilterExpression=Attr("EventType").contains('session.start' or 'kube.request'),
    )
    # Parsing the response from DynamoDB. Separating users by event type
    for item in response['Items']:
        event_type = item['FieldsMap']['event']
        user = item['FieldsMap']['user']
        if event_type in unique_users:
            # For SSH access, ensuring there is not a double count the Kube execs
            if event_type == 'session.start' and item['FieldsMap'].get('kubernetes_cluster', ''):
                continue  # skip if kubernetes_cluster exists and is not empty
            unique_users[event_type].add(user)

alias_dict = {
    'app.session.start': 'App Access',
    'db.session.start': 'DB Access',
    'kube.request': 'Kube Access',
    'session.start': 'SSH Access',
    'windows.desktop.session.start': 'Windows Access'
}
for event_type, users in unique_users.items():
    print(f'{alias_dict.get(event_type, event_type)}: {len(users)} unique users')


# By default will not print all users, to print use ENV  SHOW_USERS=YES
print_users = os.getenv("SHOW_USERS")
if print_users == "YES":
    # Print the unique users, separated by a comma and a space
    print("Printing all unique users per protocol: ")
    for event_type, users in unique_users.items():
        if len(users) != 0:
            print(f'{alias_dict.get(event_type, event_type)}: {users}')
else:
    print("If you would like to print all unique users per protocol set ENV SHOW_USERS=YES in a future run")
