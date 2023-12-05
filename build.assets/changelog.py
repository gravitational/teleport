import subprocess
import json
import os
import re

changelog_re = re.compile(r'^changelog:(.*)', re.IGNORECASE | re.MULTILINE)

base_tag = os.getenv("BASE_TAG")
base_branch = os.getenv("BASE_BRANCH")

commit = subprocess.run(
    f"git rev-list -n 1 v{base_tag}",
    shell=True, capture_output=True, text=True).stdout

date = subprocess.run(
    f"git show -s --date=format:'%Y-%m-%dT%H:%M:%S%z' --format=%cd {commit}",
    shell=True, capture_output=True, text=True).stdout

result = subprocess.run(
    f'gh pr list --search "base:{base_branch} merged:>{date} -label:no-changelog" --limit 200 --json number,title,body',
    shell=True, capture_output=True, text=True).stdout

for pr in json.loads(result):
    number = pr["number"]
    title = pr["title"]

    match = changelog_re.search(pr["body"])
    if match:
        title = match.group(1).strip()

    print(f"* {title} [#{number}](https://github.com/gravitational/teleport/pull/{number})")
