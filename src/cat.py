import sys

import command_manager
import const
import maand_data


def statement(sql, no_rows_found_msg, mode="column"):
    with open("/tmp/sql.txt", "w") as f:
        f.write(f"ATTACH DATABASE '{const.JOBS_DB_PATH}' AS job_db;\n")
        f.write(f"ATTACH DATABASE '{const.KV_DB_PATH}' AS kv_db;\n")
        f.write(".header on\n")
        f.write(f".mode {mode}\n")
        f.write(f"{sql}\n")

    with maand_data.get_db() as db:
        cursor = db.cursor()
        cursor.execute(sql)
        if len(cursor.fetchall()) == 0:
            print(no_rows_found_msg)
        else:
            command_manager.command_local(f"""
                sqlite3 {const.MAAND_DB_PATH} < /tmp/sql.txt
            """)


if __name__ == "__main__":

    name = ""
    if len(sys.argv) > 1:
        name = sys.argv[1]

    if name == "agents":
        statement(
            "SELECT agent_id, agent_ip, detained, (SELECT GROUP_CONCAT(label) FROM agent_labels al WHERE al.agent_id = a.agent_id ORDER BY label) as labels FROM agent a ORDER BY position",
            "no agents found")

    elif name == "jobs":
        statement(
            "SELECT DISTINCT job_id, name, version, (CASE WHEN (SELECT COUNT(1) FROM agent_jobs aj WHERE j.name = aj.job AND aj.disabled = 0) > 0 THEN 0 ELSE 1 END) AS disabled, deployment_seq, (SELECT GROUP_CONCAT(label) FROM job_db.job_labels jl WHERE jl.job_id = j.job_id) as labels FROM job_db.job j ORDER BY deployment_seq, name",
            "no jobs found")

    elif name == "allocations":
        statement(
            "SELECT a.agent_ip, aj.job, aj.disabled, aj.removed FROM agent a JOIN agent_jobs aj ON a.agent_id = aj.agent_id LEFT JOIN job_db.job j ON j.name = aj.job ORDER BY aj.job",
            "no allocations found")

    elif name == "alloc_commands":
        statement(
            "SELECT job_name, name as command_name, executed_on, depend_on_job, depend_on_command, depend_on_config  FROM job_commands ORDER BY job_name, name",
            "no commands found")

    elif name == "kv":
        statement(
            "SELECT * FROM (SELECT key, CASE WHEN LENGTH(value) > 50 THEN substr(value, 1, 50) || '...' ELSE value END as value, namespace, max(version) as version, ttl, created_date, deleted FROM kv_db.key_value GROUP BY key, namespace) t ORDER BY namespace, key",
            "no key values found")

    elif name == "ports":
        statement(
            "SELECT * FROM (SELECT (SELECT name FROM job WHERE job_id = jp.job_id) AS job , name, port FROM job_ports jp) t ORDER BY job, name",
            "no ports found")

    elif name == "info":
        statement(
            "SELECT bucket_id as bucket, update_seq, (SELECT (1) AS count FROM main.agent) AS 'number_of_agents', (SELECT (1) AS count FROM job_db.job) AS 'number_of_jobs' FROM bucket",
            "no info found")

    else:
        print("Usage: maand cat <operation>")
        print("Operations:")
        print("  info                   Show bucket information")
        print("  agents                 List agents")
        print("  allocations            List allocations (agents vs jobs)")
        print("  alloc_commands         List allocations commands")
        print("  kv                     List key value")
        print("  ports                  List ports")
