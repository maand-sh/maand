import maand_data
import utils
import build_agents
import build_jobs
import build_allocations
import build_variables
import build_certs
import alloc_command_executor

logger = utils.get_logger()


def post_build_hook(cursor):
    target = "post_build"
    jobs = maand_data.get_jobs(cursor)
    for job in jobs:
        maand_data.copy_job_modules(cursor, job)
        allocations = maand_data.get_allocations(cursor, job)
        job_commands = maand_data.get_job_commands(cursor, job, target)
        for command in job_commands:
            alloc_command_executor.prepare_command(cursor, job, command)
            for agent_ip in allocations:
                r = alloc_command_executor.execute_alloc_command(cursor, job, command, agent_ip, {"TARGET": target})
                if not r:
                    raise Exception(f"error job: {job}, allocation: {agent_ip}, command: {command}, error: failed with error code")

def build():
    with maand_data.get_db() as db:
        cursor = db.cursor()
        try:
            build_agents.build(cursor)
            build_jobs.build(cursor)
            build_allocations.build(cursor)
            build_variables.build(cursor)
            build_certs.build(cursor)
            db.commit()
        except Exception as e:
            db.rollback()
            raise e
        post_build_hook(cursor)
        # todo : print undefined variables


if __name__ == "__main__":
    build()
