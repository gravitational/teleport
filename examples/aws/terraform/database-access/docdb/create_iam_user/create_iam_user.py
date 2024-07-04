import pymongo
import os

def create_handler(event, context):
  docdb_endpoint = event["DOCDB_ENDPOINT"]
  docdb_master_username = event["DOCDB_MASTER_USERNAME"]
  docdb_master_password = event["DOCDB_MASTER_PASSWORD"]
  docdb_iam_user = event["DOCDB_IAM_USER"]
  docdb_url = f"mongodb://{docdb_master_username}:{docdb_master_password}@{docdb_endpoint}:27017/?tls=true&tlsInsecure=true&retryWrites=false"
  myclient = pymongo.MongoClient(docdb_url)
  myclient["$external"].command({
      "createUser": docdb_iam_user,
      "mechanisms": ["MONGODB-AWS"],
      "roles": [ { "role": "root", "db": "admin" } ],
  })
  return { "success": True }
