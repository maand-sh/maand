import argparse
import sys

import context_manager
import job_health_check
import maand_data

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--jobs', default="", required=False)
    parser.add_argument('--wait', action='store_true')
    parser.set_defaults(no_wait=True)
    args = parser.parse_args()

    if args.jobs:
        args.jobs = args.jobs.split(',')

    with maand_data.get_db() as db:
        cursor = db.cursor()

        context_manager.export_env_bucket_update_seq(cursor)
        failed = job_health_check.health_check(cursor, args.jobs, wait=args.wait)
        if failed:
            sys.exit(1)
