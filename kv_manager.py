import functools
import os.path
import sqlite3
from datetime import datetime


def put(cursor, namespace, key, value, ttl=-1):
    if type(value) is not str:
        raise TypeError("value must be a string")

    version = 0
    cursor.execute(
        "SELECT max(version), value, deleted FROM key_value WHERE namespace = ? AND key = ? GROUP BY key, namespace",
        (namespace, key),
    )
    row = cursor.fetchone()
    if row:
        version = int(row[0])
        current_value = str(row[1])
        deleted = int(row[2])
        if deleted == 0 and current_value == value:
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


def get(cursor, namespace, key):
    cursor.execute(
        "SELECT value FROM key_value WHERE namespace = ? AND key = ? AND version = (SELECT max(version) FROM key_value WHERE namespace = ? AND key = ?) AND deleted = 0",
        (namespace, key, namespace, key),
    )
    row = cursor.fetchone()
    return row[0] if row else None


def get_metadata(cursor, namespace, key):
    cursor.execute(
        "SELECT value, version FROM key_value WHERE namespace = ? AND key = ? AND version = (SELECT max(version) FROM key_value WHERE namespace = ? AND key = ?) AND deleted = 0",
        (namespace, key, namespace, key),
    )
    row = cursor.fetchone()
    return (
        (
            row[0],
            row[1],
        )
        if row
        else None
    )


def delete(cursor, namespace, key):
    cursor.execute(
        "INSERT INTO key_value (key, value, namespace, version, ttl, created_date, deleted) SELECT key, value, namespace, max(version) + 1 as version, ttl, created_date, 1 FROM key_value WHERE namespace = ? AND key = ? GROUP BY key, namespace",
        (
            namespace,
            key,
        ),
    )


def get_keys(cursor, namespace):
    cursor.execute(
        "SELECT key FROM (SELECT namespace, key, max(version), deleted, created_date FROM key_value group by key, namespace) t WHERE namespace = ? AND deleted = 0",
        (namespace,),
    )
    rows = cursor.fetchall()
    return [row[0] for row in rows]


def gc(cursor, max_days):
    cursor.execute(
        "SELECT namespace, key, max(CAST(version AS INT)) AS version, deleted, created_date FROM key_value group by key, namespace"
    )
    rows = cursor.fetchall()

    current_datetime = datetime.now()

    for row in rows:
        namespace, key, version, deleted, created_date = row

        created_datetime = datetime.fromtimestamp(int(created_date))
        date_diff = current_datetime - created_datetime

        if (
            deleted == 1 and date_diff.days >= max_days
        ):  # delete all versions if latest version is deleted and older then 15 days
            cursor.execute(
                "DELETE FROM key_value WHERE namespace = ? AND key = ?",
                (
                    namespace,
                    key,
                ),
            )

        version = version - 7
        if version < 1:
            continue
        cursor.execute(
            "DELETE FROM key_value WHERE namespace = ? AND key = ? AND version = ?",
            (namespace, key, version),
        )


def setup_global_unix_epoch():
    if not os.path.exists("/tmp/unix_epoch"):
        with sqlite3.connect(":memory:") as connection:
            c = connection.cursor()
            c.execute("SELECT unixepoch()")
            row = c.fetchone()
            with open("/tmp/unix_epoch", "w") as f:
                f.write(str(row[0]))


@functools.cache
def get_global_unix_epoch():
    with open("/tmp/unix_epoch", "r") as f:
        return f.read()


if __name__ == "__main__":
    setup_global_unix_epoch()
