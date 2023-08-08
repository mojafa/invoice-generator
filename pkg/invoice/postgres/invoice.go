package postgres

import "database/sql"

type invoiceModel struct {
	DB *sql.DB
}
