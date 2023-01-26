'''
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
'''

"""
Calculates a hash for the given subdirectories by hashing the hashes of
each file together.
"""

import hashlib, os, sys

# Make sure we're using Python3.
if sys.version_info.major < 3:
  print('This script requires Python 3 or higher. This Python version is '
        f'{sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}')
  sys.exit(1)

# Establish the root of the git repository.
repo_root = os.path.realpath(os.path.join(os.path.dirname(os.path.realpath(__file__)), ".."))

def hashfile(fullpath: str) -> bytes:
  """
  Computes a SHA256 hash of the file, which is read in blocks, and returns it.
  """
  with open(fullpath, "rb") as f:
    sha_hash = hashlib.sha256()
    for block in iter(lambda: f.read(4096), b""):
      sha_hash.update(block)
    return sha_hash.digest()

# Collect all the user provided directories. The directories are expected
# to be relative to the repo_root.
dirs = sys.argv[1:]
if len(dirs) == 0:
  print(f'Usage: {sys.argv[0]} <directories...>')
  sys.exit(1)

# Hash each file in the given directories.
hashes = {}
for directory in dirs:
  absolute_dir = os.path.join(repo_root, directory)
  if not os.path.exists(absolute_dir):
    print(f'Path {absolute_dir} does not exist.')
    sys.exit(1)

  for root, _, files in os.walk(absolute_dir):
    for name in files:
      fullpath = os.path.join(root, name)
      # Don't hash anything in node_modules, as these are build artifacts.
      if not "node_modules" in fullpath:
        hashes[fullpath] = hashfile(fullpath)

# Add in yarn.lock and package.json.
yarn_lock = os.path.join(repo_root, "yarn.lock")
package_json = os.path.join(repo_root, "package.json")
hashes[yarn_lock] = hashfile(yarn_lock)
hashes[package_json] = hashfile(package_json)

# Sort the hases so the resulting hash of hashes is the same.
sorted_hashes = sorted(hashes.items())

# Hash all of the hashes to establish a total hash for all of the subdirectories.
hash_of_hashes = hashlib.sha256()
for file_and_hash in sorted_hashes:
  hash_of_hashes.update(file_and_hash[1])

print(hash_of_hashes.hexdigest())
