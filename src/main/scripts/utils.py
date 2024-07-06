import glob
import json
import os


def custom_sort_order(element):
    custom_order_list = []
    if element in custom_order_list:
        return custom_order_list.index(element)
    else:
        return 99


def flatten(nested_list):
    flat_list = []
    stack = [nested_list]

    while stack:
        current_element = stack.pop()

        if isinstance(current_element, list):
            stack.extend(reversed(current_element))
        else:
            flat_list.append(current_element)

    return list(reversed(flat_list))


def get_hosts(host_role_filter=None):
    with open("/workspace/hosts.txt", "r") as f:
        filedata = f.read()

    lines = [x.strip() for x in filedata.split("\n") if x.strip()]

    nodes = {}
    for line in lines:
        s = line.split(" ")
        if len(s) == 2:
            nodes[s[0]] = s[1].split(",")
        if len(s) == 1:
            nodes[s[0]] = []

    for host, roles in nodes.items():
        roles = list(set(roles))
        nodes[host] = sorted(roles, key=custom_sort_order)

    if host_role_filter:
        nodes = {host: roles for host, roles in nodes.items() if set(host_role_filter) & set(roles)}

    return nodes


def get_tag_value(host, key):
    nodes = get_hosts()
    for host, roles in nodes.items():
        nodes[host] = {r.split(":")[0].strip(): r.split(":")[1].strip() for r in roles if ":" in r}
    return nodes.get(host).get(key)


def get_host_roles(host_role_filter=None):
    nodes = get_hosts(host_role_filter)
    for host, roles in nodes.items():
        nodes[host] = [r for r in roles if ":" not in r]
    return nodes


def get_host_tags(host_role_filter=None):
    nodes = get_hosts(host_role_filter)
    for host, roles in nodes.items():
        nodes[host] = {r.split(":")[0]: r.split(":")[1] for r in roles if ":" in r}
    return nodes


def get_host_one(role):
    hosts = get_host_roles()
    filtered_hosts = [ip for ip, roles in hosts.items() if role in roles]
    if len(filtered_hosts) > 0:
        return filtered_hosts[0]


def get_host_list(role):
    hosts = get_host_roles()
    return list(set([ip for ip, roles in hosts.items() if role in roles]))


def get_job_roles():
    jobs = {}
    for job in glob.glob("/workspace/jobs/*"):
        metadata_path = os.path.join(job, "metadata.json")
        if os.path.isdir(job) and os.path.isfile(metadata_path):
            with open(metadata_path, "r") as f:
                metadata = json.load(f)
                name = metadata.get("name", "")
                jobs[name] = set(metadata.get("roles"))
    return jobs
