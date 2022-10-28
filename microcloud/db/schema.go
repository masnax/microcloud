package db

import (
	"context"
	"database/sql"

	"github.com/lxc/lxd/lxd/db/schema"
)

// SchemaExtensions are additional schema updates for MicroCluster.
var SchemaExtensions = map[int]schema.Update{
	1: schemaAppend1,
}

func schemaAppend1(ctx context.Context, tx *sql.Tx) error {
	stmt := `
CREATE TABLE services (
  id               INTEGER  PRIMARY  KEY    AUTOINCREMENT  NOT  NULL,
  name             TEXT     NOT      NULL,
  state_dir        TEXT     NOT      NULL,
  UNIQUE(name)
);
  `

	_, err := tx.ExecContext(ctx, stmt)

	return err
}
