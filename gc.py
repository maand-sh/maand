import sys

from core import maand_data, utils
import kv_manager


logger = utils.get_logger()

def clean_agents(cursor):
    cursor.execute("DELETE FROM agent_labels WHERE agent_id IN (SELECT agent_id FROM agent WHERE detained = 1)")
    cursor.execute("DELETE FROM agent_tags WHERE agent_id IN (SELECT agent_id FROM agent WHERE detained = 1)")
    cursor.execute("DELETE FROM agent_jobs WHERE removed = 1")
    cursor.execute("DELETE FROM agent WHERE detained = 1")


def clean():
    with maand_data.get_db() as db:
        cursor = db.cursor()
        try:
            clean_agents(cursor)
            kv_manager.gc(cursor, -1)
            db.commit()
        except Exception as e:
            db.rollback()
            logger.fatal(e)
            sys.exit(1)


if __name__ == '__main__':
    clean()
