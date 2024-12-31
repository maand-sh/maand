import glob
import json
import os

import const


def get_agents():
    with open(f"{const.WORKSPACE_PATH}/agents.json", "r") as f:
        return json.loads(f.read())


def get_jobs():
    jobs = []
    for manifest_path in glob.glob(f"{const.WORKSPACE_PATH}/jobs/*/manifest.json"):
        job_name = os.path.basename(os.path.dirname(manifest_path))
        jobs.append(job_name)
    return jobs


def get_job_manifest(job_name):
    manifest_path = os.path.join(f"{const.WORKSPACE_PATH}/jobs", job_name, "manifest.json")
    with open(manifest_path, "r") as f:
        manifest = json.loads(f.read())
        if "labels" not in manifest:
            manifest["labels"] = []
        if "certs" not in manifest:
            manifest["certs"] = []
        if "commands" not in manifest:
            manifest["commands"] = {}
        if "resources" not in manifest:
            manifest["resources"] = {}
        return manifest


def get_job_files(job_name):
    return glob.glob("{}/**".format(job_name), recursive=True, root_dir=f"{const.WORKSPACE_PATH}/jobs")


def get_disabled_jobs():
    if not os.path.exists(f"{const.WORKSPACE_PATH}/disabled.json"):
        return {"jobs": {}, "agents": []}

    with open(f"{const.WORKSPACE_PATH}/disabled.json", "r") as f:
        return json.load(f)
