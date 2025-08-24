package data

import "database/sql"

func Event(tx *sql.Tx, short string, long string) error {
	_, err := tx.Exec("INSERT INTO events (short, long, created_date) VALUES (?, ?, strftime('%s', 'now', 'utc'))", short, long)
	if err != nil {
		return NewDatabaseError(err)
	}
	return nil
}
