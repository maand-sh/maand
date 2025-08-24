# Copyright 2014 Kiruba Sankar Swaminathan. All rights reserved.
# Use of this source code is governed by a MIT style
# license that can be found in the LICENSE file.

import argparse
import json
import os
import subprocess


def get_jobs(bucket):
    """Read and return the jobs from the JSON file for the given bucket."""
    path = f"/opt/worker/{bucket}/jobs.json"
    with open(path, "r") as file:
        jobs = json.load(file)
    return jobs


def run_job(bucket, job, command):
    """Run the make command for a single job."""
    job_path = f"/opt/worker/{bucket}/jobs/{job}"
    if os.path.exists(job_path):
        subprocess.run(["make", "-s", "-C", job_path, command], check=True)


def run_jobs(bucket, command, job_list):
    """Run the given command for each job in the job list."""
    for job in job_list:
        run_job(bucket, job, command)


def parse_args():
    """Parse and return the command-line arguments."""
    parser = argparse.ArgumentParser()
    parser.add_argument("bucket", help="Bucket name")
    parser.add_argument("cmd", help="Command to run (e.g. start, restart, etc.)")
    parser.add_argument(
        "--jobs",
        help="Comma-separated list of jobs to run (if not provided, all available jobs are used)",
        default=None
    )
    return parser.parse_args()


def main():
    args = parse_args()

    # If --jobs is given, split the string into a list
    if args.jobs:
        args.jobs = args.jobs.split(",")

    # Read jobs data from the JSON file
    jobs_data = get_jobs(args.bucket)
    all_jobs = [j.get("job") for j in jobs_data]

    # Decide which jobs to run based on the command and jobs provided
    if args.cmd in ["start", "restart"] and not args.jobs:
        # Run only the jobs that are not disabled
        jobs_to_run = [
            job.get("job") for job in jobs_data if job.get("disabled", 0) == 0
        ]
    else:
        if args.jobs:
            # Keep the order as provided in the --jobs argument and run only available jobs
            jobs_to_run = [job for job in args.jobs if job in all_jobs]
        else:
            jobs_to_run = all_jobs

    run_jobs(args.bucket, args.cmd, jobs_to_run)


if __name__ == "__main__":
    main()
