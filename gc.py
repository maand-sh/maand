from core import maand_data
import kv_manager


def clean_agents(cursor):
    cursor.execute("DELETE FROM agent_labels WHERE agent_id IN (SELECT agent_id FROM agent WHERE detained = 1)")
    cursor.execute("DELETE FROM agent_tags WHERE agent_id IN (SELECT agent_id FROM agent WHERE detained = 1)")
    cursor.execute("DELETE FROM agent WHERE detained = 1")
    cursor.execute("DELETE FROM agent_jobs WHERE removed = 1")


def clean():
    with maand_data.get_db() as db:
        cursor = db.cursor()
        clean_agents(cursor)
        kv_manager.gc(cursor, -1)
        db.commit()


if __name__ == '__main__':
    clean()
