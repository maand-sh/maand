import argparse
import sys

from core import context_manager, maand_data, job_health_check

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--jobs", default="", required=False)
    parser.add_argument("--wait", action="store_true")
    parser.set_defaults(no_wait=True)
    args = parser.parse_args()

    if args.jobs:
        args.jobs = args.jobs.split(",")

    with maand_data.get_db() as db:
        cursor = db.cursor()

        context_manager.export_env_bucket_update_seq(cursor)
        failed = job_health_check.health_check(cursor, args.jobs, wait=args.wait)
        if failed:
            sys.exit(1)
