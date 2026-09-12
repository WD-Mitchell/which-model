#!/usr/bin/env python3
"""Provision isolated administrator-policy fixtures on disposable CI runners."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


def locations():
    if sys.platform == "win32":
        parent = Path(subprocess.check_output([
            "powershell", "-NoProfile", "-Command",
            "[Environment]::GetFolderPath('CommonApplicationData')"
        ], text=True).strip())
    else:
        parent = Path("/Library/Application Support" if sys.platform == "darwin" else "/etc")
    return parent / "which-model-company-ci", parent / "which-model"


def protect(path, writable=False):
    if sys.platform == "win32":
        subprocess.run(["icacls", str(path), "/setowner", "*S-1-5-32-544"], check=True, stdout=subprocess.DEVNULL)
        inherit = "(OI)(CI)" if path.is_dir() else ""
        subprocess.run(["icacls", str(path), "/inheritance:r", "/grant:r",
                        f"*S-1-5-18:{inherit}F", f"*S-1-5-32-544:{inherit}F",
                        f"*S-1-5-32-545:{inherit}RX"], check=True, stdout=subprocess.DEVNULL)
        if writable:
            subprocess.run(["icacls", str(path), "/grant", "*S-1-1-0:W"], check=True, stdout=subprocess.DEVNULL)
    else:
        os.chown(path, 0, 0)
        os.chmod(path, 0o777 if writable else (0o755 if path.is_dir() else 0o644))
        if sys.platform == "darwin":
            subprocess.run(["chmod", "-N", str(path)], check=True)


def write(path, data):
    path.write_text(json.dumps(data) + "\n", encoding="utf-8")
    protect(path)


def prepare(env_file):
    fixture, live = locations()
    # Never overwrite an existing enrollment, even on an accidentally reused runner.
    if fixture.exists() or live.exists():
        raise RuntimeError("CI fixture locations already exist")
    fixture.mkdir()
    protect(fixture)
    (fixture / "owned-by-ci").touch()
    marker = {"schema_version": 1, "required": True}
    policy = {"schema_version": 1, "allowed_providers": []}
    for name in ("valid", "missing", "writable", "invalid", "redirect"):
        directory = fixture / name
        directory.mkdir()
        protect(directory)
        write(directory / "required.json", marker)
        if name == "missing":
            continue
        if name == "redirect":
            os.symlink(fixture / "valid" / "policy.json", directory / "policy.json")
            continue
        write(directory / "policy.json", policy if name != "invalid" else {"schema_version": 999})
        if name == "writable":
            protect(directory / "policy.json", writable=True)
    live.mkdir()
    protect(live)
    (live / "owned-by-company-ci").touch()
    managed = live / "managed"
    managed.mkdir()
    protect(managed)
    write(managed / "required.json", marker)
    write(managed / "policy.json", policy)
    with open(env_file, "a", encoding="utf-8") as env:
        env.write(f"COMPANY_POLICY_TEST_DIR={fixture}\nCOMPANY_POLICY_LIVE_TEST=1\n")


def probe(binary):
    _, live = locations()
    policy = live / "managed" / "policy.json"
    env = dict(os.environ, WHICH_MODEL_POLICY="UNTRUSTED_CANARY", PROGRAMDATA="UNTRUSTED_CANARY")
    inspected = subprocess.run([binary, "config", "policy", "--json"], env=env, text=True, capture_output=True, check=True)
    data = json.loads(inspected.stdout)
    # The canonical CLI envelope merges payload fields at the document root.
    assert data["managed"] is True and data["required"] is True, data
    assert data["policy"]["retention"] == {"usage_snapshots_hours":24,"launch_logs_days":7,"pick_history_days":30,"audit_records_days":30}
    assert "CANARY" not in inspected.stdout, inspected.stdout
    renamed = policy.with_suffix(".disabled")
    policy.rename(renamed)
    try:
        result = subprocess.run([binary, "config", "show", "--config", "UNREAD_CONFIG_CANARY"], text=True, capture_output=True)
        assert result.returncode == 2 and "company policy" in result.stderr and "CANARY" not in result.stderr, result
    finally:
        renamed.rename(policy)
    print("native CLI enrollment, environment resistance and missing-required-policy checks passed")


def cleanup():
    fixture, live = locations()
    if (live / "owned-by-company-ci").is_file():
        shutil.rmtree(live)
    if (fixture / "owned-by-ci").is_file():
        shutil.rmtree(fixture)


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise RuntimeError("this fixture may run only on disposable GitHub Actions runners")
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["prepare", "probe", "cleanup"])
    parser.add_argument("--github-env")
    parser.add_argument("--binary")
    args = parser.parse_args()
    if args.action == "prepare":
        prepare(args.github_env)
    elif args.action == "probe":
        probe(str(Path(args.binary).resolve()))
    else:
        cleanup()


if __name__ == "__main__":
    main()
