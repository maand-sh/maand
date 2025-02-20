# Copyright 2014 Kiruba Sankar Swaminathan. All rights reserved.
# Use of this source code is governed by a MIT style
# license that can be found in the LICENSE file.

import json
import os
import sqlite3


def get_job():
    return os.environ.get("JOB")


def get_command():
    return os.environ.get("COMMAND")


def get_allocation_ip():
    return os.environ.get("ALLOCATION_IP")


def get_allocation_index():
    return os.environ.get("ALLOCATION_INDEX")


def is_allocation_disabled():
    return os.environ.get("ALLOCATION_DISABLED") == "1"


def get_event():
    return os.environ.get("EVENT")


def get_db():
    return sqlite3.connect(os.environ.get("DB_PATH"))


def kv_put(cursor, namespace, key, value, ttl=0):
    if type(value) is not str:
        raise TypeError("value must be a string")

    job = get_job()
    assert key.lower() == key
    assert namespace.lower() == namespace
    assert namespace == f"vars/job/{job}"
    assert get_event() != "health_check"

    version = 0
    cursor.execute(
        "SELECT max(version), value, deleted, ttl FROM key_value WHERE namespace = ? AND key = ? GROUP BY key, namespace",
        (namespace, key),
    )
    row = cursor.fetchone()
    if row:
        version = int(row[0])
        current_value = str(row[1])
        deleted = int(row[2])
        current_ttl = int(row[3])
        if deleted == 0 and current_value == value and current_ttl == ttl:
            return
    cursor.execute(
        "INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) VALUES (?, ?, ?, ?, ?, ?, ?)",
        (
            key,
            value,
            namespace,
            version + 1,
            ttl,
            get_global_unix_epoch(),
            0,
        ),
    )


def kv_get(cursor, namespace, key):
    cursor.execute(
        "SELECT value FROM key_value WHERE namespace = ? AND key = ? AND version = (SELECT max(version) FROM key_value WHERE namespace = ? AND key = ?) AND deleted = 0",
        (namespace, key, namespace, key),
    )
    row = cursor.fetchone()
    return row[0] if row else None


def get_requesters(cursor):
    cursor.execute(
        "SELECT job, command, demand_config FROM job_commands WHERE demand_job = ? AND demand_command = ?",
        (get_job(),get_command(),),
    )
    rows = cursor.fetchall()
    reqs = []
    for row in rows:
        reqs.append({"job": row[0], "command": row[1], "config": json.loads(row[2])})
    return reqs


def get_global_unix_epoch():
    return os.environ.get("SESSION_EPOCH")
