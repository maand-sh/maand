import time

import alloc_command_executor
from core import job_data, maand_data
from core import utils


def health_check(cursor, jobs_filter, wait, interval=5, times=10):
    logger = utils.get_logger()

    jobs = job_data.get_jobs(cursor)
    if jobs_filter:
        jobs = set(jobs_filter) & set(jobs)

    def execute_health_check(job):
        result = True
        try:
            job_commands = job_data.get_job_commands(cursor, job, "health_check")
            for command in job_commands:
                alloc_command_executor.prepare_command(cursor, job, command)
                allocations = maand_data.get_allocations(cursor, job)
                for agent_ip in allocations:
                    if job not in maand_data.get_agent_removed_jobs(cursor, agent_ip):
                        result = (
                            result
                            and alloc_command_executor.execute_alloc_command(
                                job, command, agent_ip, {}
                            )
                        )
            return result
        except Exception as e:
            logger.error(f"Health check failed job : {job} and {str(e)}")
            return False

    result = True

    if wait:
        # Perform health checks with retries
        for job in jobs:
            for attempt in range(times):
                job_commands = job_data.get_job_commands(cursor, job, "health_check")
                if len(job_commands) == 0:
                    logger.info(f"Health check unknown: {job}")
                    break
                if execute_health_check(job):
                    logger.info(f"Health check succeeded: {job}")
                    break
                logger.info(
                    f"Health check failed for {job}. Retrying... ({attempt + 1}/{times})"
                )
                time.sleep(interval)
            else:
                logger.info(
                    f"Health check permanently failed for {job} after {times} retries."
                )
                result = False
    else:
        # Perform health checks without retries
        for job in jobs:
            job_commands = job_data.get_job_commands(cursor, job, "health_check")
            if len(job_commands) == 0:
                logger.info(f"Health check unknown: {job}")
                continue
            if execute_health_check(job):
                logger.info(f"Health check succeeded: {job}")
            else:
                logger.info(f"Health check failed: {job}")
                result = False

    return result
