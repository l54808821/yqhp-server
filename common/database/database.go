package database

import (
	"fmt"
	"time"

	"yqhp/common/config"
	"yqhp/common/logger"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.DatabaseConfig) error {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			cfg.Username,
			cfg.Password,
			cfg.Host,
			cfg.Port,
			cfg.Database,
			cfg.Charset,
		)
		dialector = mysql.Open(dsn)
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host,
			cfg.Port,
			cfg.Username,
			cfg.Password,
			cfg.Database,
		)
		dialector = postgres.Open(dsn)
	default:
		return fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	var err error
	db, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.NewGormLogger(),
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	return nil
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return db
}

// Close 关闭数据库连接
func Close() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
