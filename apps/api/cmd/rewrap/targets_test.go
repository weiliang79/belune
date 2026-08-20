package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/testutil"
)

// TestTargetsCoverEveryEncryptedColumn asks the schema, not a list someone has
// to remember to update. Forgetting a column here is silent in the worst way:
// rewrap reports success, the operator retires the old KEK believing every
// secret has been re-sealed, and the ones it skipped can no longer be
// decrypted. The failure surfaces months later, on a key rotation, with no
// path back.
//
// This is the same shape as internal/config's documented_test: the fix for
// "someone will forget" is something that fails when the two disagree.
func TestTargetsCoverEveryEncryptedColumn(t *testing.T) {
	pool, _, teardown := testutil.SetupTestDB()
	defer teardown()

	rows, err := pool.Query(context.Background(), `
		SELECT table_name, column_name
		FROM   information_schema.columns
		WHERE  table_schema = 'public' AND column_name LIKE '%\_encrypted'
		ORDER  BY table_name, column_name
	`)
	require.NoError(t, err)
	defer rows.Close()

	covered := make(map[string]bool, len(targets))
	for _, tgt := range targets {
		covered[tgt.table+"."+tgt.colEnc] = true
	}

	var found []string
	for rows.Next() {
		var table, column string
		require.NoError(t, rows.Scan(&table, &column))
		found = append(found, table+"."+column)
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, found, "no *_encrypted columns found — the query or the schema is wrong")

	for _, col := range found {
		assert.True(t, covered[col],
			"%s is encrypted but rewrap does not rewrap it: key rotation would skip it and "+
				"retiring the old KEK would make it undecryptable. Add it to targets.", col)
	}

	// The converse: a target naming a column that no longer exists would fail at
	// runtime, mid-rotation, on a half-rewrapped database.
	inSchema := make(map[string]bool, len(found))
	for _, col := range found {
		inSchema[col] = true
	}
	for _, tgt := range targets {
		name := tgt.table + "." + tgt.colEnc
		assert.True(t, inSchema[name], "rewrap targets %s, which is not in the schema", name)
	}

	fmt.Printf("rewrap covers %d encrypted columns\n", len(found))
}
