// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"database/sql"
	"errors"

	"maand/bucket"
)

// AllocationFileManifests holds staged and promoted per-file digests for an allocation.
type AllocationFileManifests struct {
	Current         FileManifest
	Previous        FileManifest
	HasCurrentFiles bool
	HasPreviousFiles bool
}

// GetAllocationFileManifests returns staged and promoted file manifests for an allocation.
func GetAllocationFileManifests(tx *sql.Tx, namespace, key string) (AllocationFileManifests, error) {
	var currentRaw, previousRaw sql.NullString
	row := tx.QueryRow(
		`SELECT current_files, previous_files FROM hash WHERE namespace = ? AND key = ?`,
		namespace, key,
	)
	err := row.Scan(&currentRaw, &previousRaw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AllocationFileManifests{}, nil
		}
		return AllocationFileManifests{}, bucket.DatabaseError(err)
	}

	current, hasCurrent, err := ParseFileManifest(currentRaw)
	if err != nil {
		return AllocationFileManifests{}, err
	}
	previous, hasPrevious, err := ParseFileManifest(previousRaw)
	if err != nil {
		return AllocationFileManifests{}, err
	}
	return AllocationFileManifests{
		Current:          current,
		Previous:         previous,
		HasCurrentFiles:  hasCurrent,
		HasPreviousFiles: hasPrevious,
	}, nil
}
