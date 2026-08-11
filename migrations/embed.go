// Package migrations embeds the SQL migration files so the running service can
// verify that the database schema is up to date.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
