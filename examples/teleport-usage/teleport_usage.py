import json
import datetime
import boto3
import time
import os

from boto3.dynamodb.conditions import Key, Attr

# Check for AWS DynamoDB table name and region ENV, if none prompt
TABLE_NAME = os.getenv("TABLE_NAME")
if TABLE_NAME == None:
    TABLE_NAME = input("DynamoDB events table name: ")

aws_region = os.getenv("AWS_REGION")
if aws_region == None:
    aws_region = input("AWS Region for DynamoDB: ")

# Check ENV for start and end times, if none, set last 30 days
start_date = os.getenv("START_DATE")
if start_date == None:
    now = datetime.datetime.now()
    start_date = now - datetime.timedelta(days=30)
    
end_date = os.getenv("END_DATE")
if end_date == None:
    end_date = now

# Configure DynamoDB for query
dynamodb = boto3.resource("dynamodb", region_name=aws_region)
table = dynamodb.Table(TABLE_NAME)

# variables for protocols
app = 'app.session.chunk'
db = 'db.session.start'
kube = 'kube.request'
ssh = 'session.start'
win = 'windows.desktop.session.start'

protocols = ['app', 'db', 'kube', 'win']
all_users = []

# Generating data message
start_date_str = start_date.strftime("%b %d %y")
end_date_str = end_date.strftime("%b %d %y")
print ("""

NOTE: By default this tool queries usage for the last 30 days") 
For a custom date range use ENV START_DATE and END_DATE (format YYYY-MM-DD)")

""")
print(f"Gathering data for unique users who accessed each protocol between {start_date_str} to {end_date_str}: ")

unique_users = set()

# Run Dynamodb query to retrieve all login for SSH for specified date range
for i in range((end_date - start_date).days + 1):
    current_date = start_date + datetime.timedelta(i)
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
if len(unique_users) == 0:
  unique_users = '0'
else:
  all_users.append(f"ssh: {unique_users}")

# Loop through the protocols in the list
for protocol in protocols:
  # Create a set for the unique users of this protocol
  unique_users = set()

  # Run Dynamodb query to retrieve all login event types for specified date range
  for i in range((end_date - start_date).days + 1):
      current_date = start_date + datetime.timedelta(i)
      date = current_date.strftime("%Y-%m-%d")
      # print(date)
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
  if len(unique_users) == 0:
    unique_users = '0'
  else:
    all_users.append(f"{protocol}: {unique_users}")

# By default will print all users, to suppress use ENV  HIDE_USERS=YES
print(""" 
Printing all unique users per protocol. 
To suppress users in the future set ENV HIDE_USERS=YES """)

print_users = os.getenv("HIDE_USERS")
if print_users != "YES":
    # Print the unique users, separated by a comma and a space
    print(json.dumps(all_users, indent=4, separators=(',', ':')))
