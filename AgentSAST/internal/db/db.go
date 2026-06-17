// Package db opens the MySQL connection and ensures the shared schema.
package db

import (
	"errors"
	"strings"

	"agentsast/internal/db/model"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dsn string) (*gorm.DB, error) {
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	if err := migrateSAST(gdb); err != nil {
		return nil, err
	}
	return gdb, nil
}

// migrateSAST adds missing columns on the shared `sast` table. Manager may create
// or extend the table first; GORM AutoMigrate can emit duplicate-column ALTERs.
func migrateSAST(gdb *gorm.DB) error {
	m := gdb.Migrator()
	dst := &model.SAST{}
	if !m.HasTable(dst) {
		return m.CreateTable(dst)
	}

	stmt := &gorm.Statement{DB: gdb}
	if err := stmt.Parse(dst); err != nil {
		return err
	}
	for _, field := range stmt.Schema.Fields {
		if field.IgnoreMigration || field.DBName == "" {
			continue
		}
		if m.HasColumn(dst, field.DBName) {
			continue
		}
		if err := m.AddColumn(dst, field.Name); err != nil {
			if isDuplicateColumnErr(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	var me *mysqldriver.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1060
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column")
}

// Ping verifies connectivity (used by /health).
func Ping(gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
