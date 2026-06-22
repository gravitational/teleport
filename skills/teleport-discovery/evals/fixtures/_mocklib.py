#!/usr/bin/env python3
"""Shared dispatcher for the eval stub CLIs (tctl, tsh, aws, az).

Each fixture's bin/<cli> is a thin shim that calls main("<cli>", argv). The
scenario to emulate is selected by the MOCK_SCENARIO env var, an absolute path
to a JSON file under evals/fixtures/scenarios/. prep.py sets MOCK_SCENARIO per
run from the eval's `fixture` field, so one shared bin serves every scenario.

The emulated outputs match exactly what the skill reads:
- tctl status prints a `Version:` line (cluster_version derivation).
- tsh status --format=json exposes active.profile_url (proxy_addr derivation).
- tctl get discovery_config --format=json returns the resource with a
  protojson status: .status.state and
  .status.integrationDiscoveredResources.<integration>.{awsEc2|awsEks|azureVms}.
- tctl get user_tasks --format=json returns spec.state / spec.task_type /
  spec.issue_type per task.
- aws iam get-open-id-connect-provider returns a bare-host .Url for the
  existing-provider match.
- az group show --query location --output tsv prints the group location, or
  exits non-zero when the group does not exist.

A command the scenario does not configure exits non-zero with a clear message,
so a misrouted call fails loudly instead of returning misleading data.
"""

from __future__ import annotations

import json
import os
import sys


def _load_scenario() -> dict:
    path = os.environ.get("MOCK_SCENARIO")
    if not path:
        sys.stderr.write(
            "mock CLI: MOCK_SCENARIO is not set; the eval harness must export it.\n"
        )
        sys.exit(2)
    try:
        with open(path) as f:
            return json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        sys.stderr.write(f"mock CLI: cannot read MOCK_SCENARIO {path}: {e}\n")
        sys.exit(2)


def _wants_json(args: list[str]) -> bool:
    if "--format=json" in args:
        return True
    if _flag_value(args, "--format") == "json":
        return True
    if _flag_value(args, "-o") == "json" or _flag_value(args, "--output") == "json":
        return True
    return False


def _flag_value(args: list[str], flag: str) -> str | None:
    """Value of `--flag value` or `--flag=value`."""
    for i, a in enumerate(args):
        if a == flag and i + 1 < len(args):
            return args[i + 1]
        if a.startswith(flag + "="):
            return a.split("=", 1)[1]
    return None


def _out(obj) -> None:
    if isinstance(obj, str):
        print(obj)
    else:
        print(json.dumps(obj))


def _unconfigured(cli: str, args: list[str]) -> None:
    sys.stderr.write(
        f"mock {cli}: scenario does not configure `{cli} {' '.join(args)}`\n"
    )
    sys.exit(1)


# --------------------------------------------------------------- tsh


def _tsh(scn: dict, args: list[str]) -> None:
    tsh = scn.get("tsh", {})
    if not args or args[0] != "status":
        _unconfigured("tsh", args)
    if tsh.get("unauth"):
        proxy = tsh.get("proxy", "<proxy-addr>")
        sys.stderr.write(f"ERROR: Not logged in. Run: tsh login --proxy={proxy}\n")
        sys.exit(1)
    profile = {
        "profile_url": tsh["profile_url"],
        "cluster": tsh.get("cluster", ""),
        "username": tsh.get("username", "eval"),
        "roles": tsh.get("roles", ["access", "editor"]),
        "logins": tsh.get("logins", ["ubuntu"]),
        "traits": tsh.get("traits", {"logins": ["ubuntu"]}),
        "valid_until": tsh.get("valid_until", "2026-12-01T00:00:00Z"),
    }
    if _wants_json(args):
        _out({"active": profile, "profiles": [profile]})
    else:
        _out(
            f"Profile URL:   {profile['profile_url']}\n"
            f"Logged in as:  {profile['username']}\n"
            f"Cluster:       {profile['cluster']}\n"
            f"Roles:         {', '.join(profile['roles'])}\n"
            f"Logins:        {', '.join(profile['logins'])}"
        )


# --------------------------------------------------------------- tctl


def _tctl(scn: dict, args: list[str]) -> None:
    tctl = scn.get("tctl", {})
    if not args:
        sys.stderr.write("ERROR: no command given\n")
        sys.exit(1)
    cmd = args[0]

    if cmd == "status":
        _out(
            f"Cluster:     {tctl.get('cluster', '')}\n"
            f"Version:     {tctl.get('version', '')}\n"
            f"CA pin:      sha256:0000000000000000000000000000000000000000000000000000000000000000\n"
            f"Proxy:       {tctl.get('cluster', '')}:443"
        )
        return

    if cmd == "version":
        if _wants_json(args) or "-f" in args:
            _out(
                {
                    "version": f"v{tctl.get('version', '')}",
                    "gitRef": "eval",
                    "hostname": tctl.get("cluster", ""),
                }
            )
        else:
            _out(f"Teleport v{tctl.get('version', '')} git:eval go1.22.0")
        return

    if cmd == "get":
        resource = args[1] if len(args) > 1 else ""
        base = resource.split("/", 1)[0]
        if base == "integrations":
            data = tctl.get("integrations", [])
        elif base == "discovery_config":
            dc = tctl.get("discovery_config")
            data = [dc] if dc else []
        elif base == "user_tasks":
            data = tctl.get("user_tasks", [])
        else:
            sys.stderr.write(f"ERROR: unknown resource: {resource}\n")
            sys.exit(1)
        if _wants_json(args):
            _out(data)
        else:
            sys.stderr.write(f"ERROR: {resource}: use --format=json\n")
            sys.exit(1)
        return

    if cmd == "discovery":
        if len(args) < 2 or args[1] != "nodes":
            sys.stderr.write(f"ERROR: unknown discovery subcommand: {args[1:]}\n")
            sys.exit(1)
        nodes = tctl.get("discovery_nodes", [])
        if _wants_json(args):
            _out(nodes)
        else:
            lines = [f"{'Node Name':<20} {'Status':<28} Last Seen", "-" * 68]
            for n in nodes:
                lines.append(
                    f"{n['name']:<20} {n['status']:<28} {n.get('last_seen', '')}"
                )
            _out("\n".join(lines))
        return

    if cmd == "tokens":
        if len(args) > 1 and args[1] == "ls":
            tokens = tctl.get("tokens", [])
            lines = ["Token                            Type        Expiry", "-" * 52]
            for t in tokens:
                lines.append(
                    f"{t['name']:<32} {t.get('type', 'iam,node'):<11} {t.get('expiry', 'never')}"
                )
            _out("\n".join(lines))
            return
        _unconfigured("tctl", args)

    if cmd == "inventory":
        if len(args) > 1 and args[1] == "list":
            services = tctl.get("inventory_discovery", [])
            if not services:
                _out("")
            else:
                lines = [
                    "Server ID                            Services   Version",
                    "-" * 60,
                ]
                for s in services:
                    lines.append(
                        f"{s.get('id', 'eval'):<36} discovery  {tctl.get('version', '')}"
                    )
                _out("\n".join(lines))
            return
        _unconfigured("tctl", args)

    if cmd == "terraform":
        if len(args) > 1 and args[1] == "env":
            _out('export TF_TOKEN_terraform_releases_teleport_dev="mock-token-eval"')
            return
        _unconfigured("tctl", args)

    sys.stderr.write(f"ERROR: unknown command: {cmd}\n")
    sys.exit(1)


# --------------------------------------------------------------- aws


def _aws(scn: dict, args: list[str]) -> None:
    aws = scn.get("aws")
    if aws is None:
        _unconfigured("aws", args)
    joined = " ".join(args)

    if "get-caller-identity" in joined:
        if (
            _flag_value(args, "--query") == "Account"
            and _flag_value(args, "--output") == "text"
        ):
            _out(aws["account"])
        else:
            _out(
                {
                    "UserId": "AIDAEXAMPLE",
                    "Account": aws["account"],
                    "Arn": aws.get(
                        "caller_arn", f"arn:aws:iam::{aws['account']}:user/eval"
                    ),
                }
            )
        return

    if "list-open-id-connect-providers" in joined:
        providers = [{"Arn": p["arn"]} for p in aws.get("oidc_providers", [])]
        _out({"OpenIDConnectProviderList": providers})
        return

    if "get-open-id-connect-provider" in joined:
        arn = _flag_value(args, "--open-id-connect-provider-arn")
        for p in aws.get("oidc_providers", []):
            if p["arn"] == arn:
                _out(
                    {
                        "Url": p["url"],
                        "ClientIDList": ["discover.teleport"],
                        "ThumbprintList": [],
                        "CreateDate": "2025-01-15T00:00:00+00:00",
                    }
                )
                return
        sys.stderr.write(f"An error occurred (NoSuchEntity): no OIDC provider {arn}\n")
        sys.exit(254)

    if "configure" in joined and "get" in joined and "region" in joined:
        _out(aws.get("region", "us-east-1"))
        return

    _unconfigured("aws", args)


# --------------------------------------------------------------- az


def _az(scn: dict, args: list[str]) -> None:
    az = scn.get("az")
    if az is None:
        _unconfigured("az", args)
    if len(args) < 2:
        _unconfigured("az", args)
    cmd, sub = args[0], args[1]

    if cmd == "account" and sub == "show":
        if (
            _flag_value(args, "--query") == "id"
            and _flag_value(args, "--output") == "tsv"
        ):
            _out(az["subscription_id"])
        else:
            _out(
                {
                    "id": az["subscription_id"],
                    "name": "eval-subscription",
                    "state": "Enabled",
                }
            )
        return

    if cmd == "account" and sub == "list-locations":
        locs = [
            {"name": "eastus", "displayName": "East US"},
            {"name": "westus", "displayName": "West US"},
        ]
        _out(locs)
        return

    if cmd == "group" and sub == "show":
        name = _flag_value(args, "--name") or _flag_value(args, "-n")
        groups = az.get("resource_groups", {})
        if name not in groups:
            sys.stderr.write(
                f"ERROR: (ResourceGroupNotFound) Resource group '{name}' could not be found.\n"
            )
            sys.exit(3)
        location = groups[name]
        if (
            _flag_value(args, "--query") == "location"
            and _flag_value(args, "--output") == "tsv"
        ):
            _out(location)
        else:
            _out(
                {
                    "name": name,
                    "location": location,
                    "properties": {"provisioningState": "Succeeded"},
                }
            )
        return

    _unconfigured("az", args)


def main(cli: str, args: list[str]) -> None:
    scn = _load_scenario()
    {"tsh": _tsh, "tctl": _tctl, "aws": _aws, "az": _az}[cli](scn, args)
