import maand
import utils
import build_agents
import build_jobs
import build_allocations
import build_variables
import build_certs
import alloc_command_executor

logger = utils.get_logger()


def hook(cursor, when):
    target = "build"
    jobs = maand.get_jobs(cursor)
    for job in jobs:
        maand.copy_job_modules(cursor, job)
        allocations = maand.get_allocations(cursor, job)
        job_commands = maand.get_job_commands(cursor, job, f"{when}_{target}")
        for command in job_commands:
            alloc_command_executor.prepare_command(cursor, job, command)
            for agent_ip in allocations:
                alloc_command_executor.execute_alloc_command(cursor, job, command, agent_ip, {"TARGET": target})

def build():
    with maand.get_db() as db:
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
        hook(cursor, "post")
        # todo : print undefined variables


if __name__ == "__main__":
    build()
