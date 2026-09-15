package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/guimc233/JustPing/host/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB initializes PostgreSQL connection, runs migrations, and seeds baseline configuration
func InitDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "postgres")
		pass := getEnv("DB_PASSWORD", "postgres")
		name := getEnv("DB_NAME", "justping")
		ssl := getEnv("DB_SSLMODE", "disable")
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
			host, port, user, pass, name, ssl)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = DB.AutoMigrate(
		&model.SystemSetting{},
		&model.EmailWhitelist{},
		&model.User{},
		&model.Target{},
		&model.Agent{},
		&model.PingMetric{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-migrate database: %w", err)
	}

	// Seed baseline settings (is_initialized & jwt_secret) atomically so FOR UPDATE has a row
	if err := seedBaselineSettings(DB); err != nil {
		return nil, fmt.Errorf("failed to seed baseline settings: %w", err)
	}

	log.Println("[DB] Database connected and schema migrated successfully")
	return DB, nil
}

func seedBaselineSettings(tx *gorm.DB) error {
	// 1. Ensure is_initialized exists
	initSetting := model.SystemSetting{
		Key:       "is_initialized",
		Value:     "false",
		UpdatedAt: time.Now(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&initSetting).Error; err != nil {
		return err
	}

	// 2. Ensure jwt_secret is initialized consistently across replicas
	envSecret := os.Getenv("JWT_SECRET")
	if envSecret != "" {
		_ = SetSetting("jwt_secret", envSecret)
	} else {
		var s model.SystemSetting
		if err := tx.Where("key = ?", "jwt_secret").First(&s).Error; err != nil {
			b := make([]byte, 32)
			_, _ = rand.Read(b)
			genSecret := hex.EncodeToString(b)
			newSecret := model.SystemSetting{Key: "jwt_secret", Value: genSecret, UpdatedAt: time.Now()}
			_ = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&newSecret).Error
		}
	}
	return nil
}

// GetSetting retrieves a system setting by key
func GetSetting(key string) string {
	var s model.SystemSetting
	if err := DB.Where("key = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// SetSetting saves or updates a system setting
func SetSetting(key, value string) error {
	s := model.SystemSetting{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
	}
	return DB.Save(&s).Error
}

// StartRetentionCleaner periodically cleans up old ping metrics
func StartRetentionCleaner(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			retentionDays := 30
			retentionStr := GetSetting("retention_days")
			if retentionStr != "" {
				var d int
				if _, err := fmt.Sscanf(retentionStr, "%d", &d); err == nil && d > 0 {
					retentionDays = d
				}
			}
			cutoff := time.Now().AddDate(0, 0, -retentionDays)
			res := DB.Where("timestamp < ?", cutoff).Delete(&model.PingMetric{})
			if res.Error != nil {
				log.Printf("[DB] Retention cleaner error: %v\n", res.Error)
			} else if res.RowsAffected > 0 {
				log.Printf("[DB] Retention cleaner pruned %d metrics older than %d days\n", res.RowsAffected, retentionDays)
			}
		}
	}()
}

func getEnv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}
