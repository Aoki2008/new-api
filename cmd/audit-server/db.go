package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDB(cfg Config) (*gorm.DB, *sql.DB, error) {
	var dialector gorm.Dialector
	switch strings.ToLower(strings.TrimSpace(cfg.DBDriver)) {
	case "", "sqlite", "sqlite3":
		dialector = sqlite.Open(cfg.DBDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DBDSN)
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.DBDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported AUDIT_DB_DRIVER: %s", cfg.DBDriver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := db.AutoMigrate(&AuditRecord{}); err != nil {
		return nil, nil, err
	}
	return db, sqlDB, nil
}
