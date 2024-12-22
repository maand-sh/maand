import multiprocessing
import sys

import command_helper
import const
import maand


def split_list(input_list, chunk_size=3):
    return [
        input_list[i : i + chunk_size] for i in range(0, len(input_list), chunk_size)
    ]

def run(cursor, func, agents=None, concurrency=None, labels_filter=None, agents_filter=None):
    agents = agents or maand.get_agents(cursor, labels_filter)

    if agents_filter:
        agents = list(set(agents_filter) & set(agents))

    command_helper.command_local(f"mkdir -p {const.LOGS_FOLDER}")
    for agent in agents:
        with open(f"{const.LOGS_FOLDER}/{agent}.log", "w") as log_file:
            log_file.flush()

    inputs = []
    for agent in agents:
        inputs.append(agent)

    if len(agents) == 0:
        sys.exit(0)

    work_items = split_list(inputs, chunk_size=concurrency or len(inputs))
    for work_item in work_items:
        with multiprocessing.Pool(processes=len(work_item)) as pool:
            pool.map(func, work_item)
