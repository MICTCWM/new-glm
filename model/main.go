package model

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string

func initCol() {
	// init common column names
	if common.UsingPostgreSQL {
		commonGroupCol = `"group"`
		commonKeyCol = `"key"`
		commonTrueVal = "true"
		commonFalseVal = "false"
	} else {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch common.LogSqlType {
		case common.DatabaseTypePostgreSQL:
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		default:
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	} else {
		// LOG_SQL_DSN 为空时，日志数据库与主数据库相同
		if common.UsingPostgreSQL {
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		} else {
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	}
	// log sql type and database type
	//common.SysLog("Using Log SQL Type: " + common.LogSqlType)
}

var DB *gorm.DB

var LOG_DB *gorm.DB

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CheckSetup() {
	setup := GetSetup()
	if setup == nil {
		// No setup record exists, check if we have a root user
		if RootUserExists() {
			common.SysLog("system is not initialized, but root user exists")
			// Create setup record
			newSetup := Setup{
				Version:       common.Version,
				InitializedAt: time.Now().Unix(),
			}
			err := DB.Create(&newSetup).Error
			if err != nil {
				common.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			common.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
	} else {
		// Setup record exists, system is initialized
		common.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
		constant.Setup = true
	}
}

func chooseDB(envName string, isLog bool) (*gorm.DB, error) {
	defer func() {
		initCol()
	}()
	dsn := os.Getenv(envName)
	if dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			common.SysLog("using PostgreSQL as database")
			if !isLog {
				common.UsingPostgreSQL = true
			} else {
				common.LogSqlType = common.DatabaseTypePostgreSQL
			}
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		if strings.HasPrefix(dsn, "local") {
			common.SysLog("SQL_DSN not set, using SQLite as database")
			if !isLog {
				common.UsingSQLite = true
			} else {
				common.LogSqlType = common.DatabaseTypeSQLite
			}
			return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		// Use MySQL
		common.SysLog("using MySQL as database")
		// check parseTime
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if !isLog {
			common.UsingMySQL = true
		} else {
			common.LogSqlType = common.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	common.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB("SQL_DSN", false)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		// MySQL charset/collation startup check: ensure Chinese-capable charset
		if common.UsingMySQL {
			if err := checkMySQLChineseSupport(DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		idleConns, openConns := databasePoolDefaults(false)
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", idleConns))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", openConns))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		if common.UsingMySQL {
			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		if !common.IsMasterNode {
			return nil
		}
		return LOG_DB.Transaction(func(tx *gorm.DB) error {
			return migrateSpecialUsageSchema(tx)
		})
	}
	db, err := chooseDB("LOG_SQL_DSN", true)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		LOG_DB = db
		// If log DB is MySQL, also ensure Chinese-capable charset
		if common.LogSqlType == common.DatabaseTypeMySQL {
			if err := checkMySQLChineseSupport(LOG_DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := LOG_DB.DB()
		if err != nil {
			return err
		}
		idleConns, openConns := databasePoolDefaults(true)
		sqlDB.SetMaxIdleConns(getDBPoolEnv("LOG_SQL_MAX_IDLE_CONNS", "SQL_MAX_IDLE_CONNS", idleConns))
		sqlDB.SetMaxOpenConns(getDBPoolEnv("LOG_SQL_MAX_OPEN_CONNS", "SQL_MAX_OPEN_CONNS", openConns))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(getDBPoolEnv("LOG_SQL_MAX_LIFETIME", "SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func databasePoolDefaults(isLog bool) (idleConns, openConns int) {
	if (!isLog && common.UsingSQLite) || (isLog && common.LogSqlType == common.DatabaseTypeSQLite) {
		return 1, 4
	}
	return 100, 100
}

func getDBPoolEnv(name, fallback string, defaultValue int) int {
	if os.Getenv(name) != "" {
		return common.GetEnvOrDefault(name, defaultValue)
	}
	return common.GetEnvOrDefault(fallback, defaultValue)
}

func migrateDB() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return migrateDBWith(tx)
	})
}

func migrateDBWith(db *gorm.DB) error {
	// Migrate price_amount column from float/double to decimal for existing tables
	if err := migrateSubscriptionPlanPriceAmount(db); err != nil {
		return err
	}
	// Migrate model_limits column from varchar to text for existing tables
	if err := migrateTokenModelLimitsToText(db); err != nil {
		return err
	}
	if err := migrateUserGptQuotaPrecision(db); err != nil {
		return err
	}

	err := db.AutoMigrate(
		&Channel{},
		&ChannelResetRule{},
		&Token{},
		&User{},
		&PasskeyCredential{},
		&Option{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Midjourney{},
		&TopUp{},
		&QuotaData{},
		&Task{},
		&Model{},
		&Vendor{},
		&PrefillGroup{},
		&Setup{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Checkin{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionUsageBucket{},
		&SubscriptionPreConsumeRecord{},
		&CustomOAuthProvider{},
		&UserOAuthBinding{},
		&PerfMetric{},
		&SubscriptionRedemption{},
		&Ticket{},
		&TicketImage{},
		&TicketReply{},
		&LogDetail{},
		&VisionRouteImageCache{},
	)
	if err != nil {
		return err
	}
	if err := ensureUserGptQuotaColumn(db); err != nil {
		return err
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(db); err != nil {
			return err
		}
	} else {
		if err := db.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	return nil
}

func migrateDBFast() error {
	// Keep the legacy entry point, but never run DDL concurrently. SQLite has a
	// single writer and partial parallel migrations can leave mixed schema state.
	return migrateDB()
}

func ensureUserGptQuotaColumn(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasColumn(&User{}, "GptQuota") {
		return db.Migrator().AddColumn(&User{}, "GptQuota")
	}
	return migrateUserGptQuotaPrecision(db)
}

func migrateUserGptQuotaPrecision(db *gorm.DB) error {
	if common.UsingSQLite {
		return nil
	}

	const tableName = "users"
	const columnName = "gpt_quota"
	requiredPrecision := 36
	requiredScale := userGptQuotaScale

	if !db.Migrator().HasTable(tableName) {
		return nil
	}
	if !db.Migrator().HasColumn(&User{}, "GptQuota") {
		return nil
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		var meta struct {
			DataType         string `gorm:"column:data_type"`
			NumericPrecision int    `gorm:"column:numeric_precision"`
			NumericScale     int    `gorm:"column:numeric_scale"`
		}
		if err := db.Raw(`SELECT data_type,
				COALESCE(numeric_precision, 0) AS numeric_precision,
				COALESCE(numeric_scale, 0) AS numeric_scale
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&meta).Error; err != nil {
			return fmt.Errorf("failed to query metadata for %s.%s: %w", tableName, columnName, err)
		}
		if strings.EqualFold(meta.DataType, "numeric") &&
			meta.NumericPrecision >= requiredPrecision &&
			meta.NumericScale >= requiredScale {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s`,
			tableName, columnName, userGptQuotaColumnType, columnName, userGptQuotaColumnType)
	} else if common.UsingMySQL {
		var meta struct {
			DataType         string `gorm:"column:data_type"`
			NumericPrecision int    `gorm:"column:numeric_precision"`
			NumericScale     int    `gorm:"column:numeric_scale"`
		}
		if err := db.Raw(`SELECT DATA_TYPE AS data_type,
				COALESCE(NUMERIC_PRECISION, 0) AS numeric_precision,
				COALESCE(NUMERIC_SCALE, 0) AS numeric_scale
			FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
			tableName, columnName).Scan(&meta).Error; err != nil {
			return fmt.Errorf("failed to query metadata for %s.%s: %w", tableName, columnName, err)
		}
		if strings.EqualFold(meta.DataType, "decimal") &&
			meta.NumericPrecision >= requiredPrecision &&
			meta.NumericScale >= requiredScale {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s DEFAULT 0",
			tableName, columnName, userGptQuotaColumnType)
	} else {
		return nil
	}

	if err := db.Exec(alterSQL).Error; err != nil {
		return fmt.Errorf("failed to migrate %s.%s to %s: %w", tableName, columnName, userGptQuotaColumnType, err)
	}
	common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to %s", tableName, columnName, userGptQuotaColumnType))
	return nil
}

func migrateLOGDB() error {
	return LOG_DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&Log{}, &LogDetail{}); err != nil {
			return err
		}
		return migrateSpecialUsageSchema(tx)
	})
}

// migrateSpecialUsageSchema upgrades the independent usage ledger without
// allowing the legacy request_id+channel_id unique index to discard retry
// attempts. AutoMigrate adds new columns/indexes, then we normalize duplicate
// legacy attempts before recreating the new composite index.
func migrateSpecialUsageSchema(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&SpecialUsageRecord{}) {
		return db.AutoMigrate(&SpecialUsageRecord{}, &SpecialUsageHourly{})
	}

	// Remove either generation of the unique index before adding the new
	// columns. The legacy index makes all old rows use attempt=0, so creating
	// the new index before normalizing those rows would fail on startup.
	for _, indexName := range []string{
		"uk_special_usage_request_attempt",
		"uk_special_usage_request_channel",
	} {
		if db.Migrator().HasIndex(&SpecialUsageRecord{}, indexName) {
			if err := db.Migrator().DropIndex(&SpecialUsageRecord{}, indexName); err != nil {
				return fmt.Errorf("drop special usage index %s: %w", indexName, err)
			}
		}
	}
	for _, field := range []string{"Attempt", "UsageSource", "PriceSnapshot"} {
		if !db.Migrator().HasColumn(&SpecialUsageRecord{}, field) {
			if err := db.Migrator().AddColumn(&SpecialUsageRecord{}, field); err != nil {
				return fmt.Errorf("add special usage column %s: %w", field, err)
			}
		}
	}

	var rows []struct {
		ID        int64
		RequestID string
		ChannelID int
		Attempt   int
	}
	if err := db.Model(&SpecialUsageRecord{}).
		Select("id, request_id, channel_id, attempt").
		Order("id asc").Find(&rows).Error; err != nil {
		return fmt.Errorf("read special usage attempts: %w", err)
	}
	seen := make(map[string]map[int]struct{})
	for _, row := range rows {
		key := fmt.Sprintf("%s\\x00%d", row.RequestID, row.ChannelID)
		if seen[key] == nil {
			seen[key] = make(map[int]struct{})
		}
		desired := row.Attempt
		if _, exists := seen[key][desired]; exists {
			desired = 0
			for {
				if _, exists := seen[key][desired]; !exists {
					break
				}
				desired++
			}
		}
		seen[key][desired] = struct{}{}
		if desired != row.Attempt {
			if err := db.Model(&SpecialUsageRecord{}).Where("id = ?", row.ID).Update("attempt", desired).Error; err != nil {
				return fmt.Errorf("normalize special usage attempt %d: %w", row.ID, err)
			}
		}
	}
	if err := db.AutoMigrate(&SpecialUsageRecord{}, &SpecialUsageHourly{}); err != nil {
		return err
	}
	return nil
}

type sqliteColumnDef struct {
	Name string
	DDL  string
}

func ensureSubscriptionPlanTableSQLite(db *gorm.DB) error {
	if !common.UsingSQLite {
		return nil
	}
	tableName := "subscription_plans"
	if !db.Migrator().HasTable(tableName) {
		createSQL := `CREATE TABLE ` + "`" + tableName + "`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` numeric NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`accessible_groups`" + ` json,
` + "`restricted_groups`" + ` json,
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`weekly_amount_limit`" + ` bigint NOT NULL DEFAULT 0,
` + "`special_quota_enabled`" + ` numeric DEFAULT 0,
` + "`hourly_reset_hours`" + ` integer DEFAULT 5,
` + "`hourly_amount_limit`" + ` bigint NOT NULL DEFAULT 0,
` + "`special_weekly_reset_weeks`" + ` integer DEFAULT 1,
` + "`special_weekly_amount_limit`" + ` bigint NOT NULL DEFAULT 0,
` + "`special_config_updated_at`" + ` bigint DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
		return db.Exec(createSQL).Error
	}
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "title", DDL: "`title` varchar(128) NOT NULL"},
		{Name: "subtitle", DDL: "`subtitle` varchar(255) DEFAULT ''"},
		{Name: "price_amount", DDL: "`price_amount` numeric NOT NULL"},
		{Name: "currency", DDL: "`currency` varchar(8) NOT NULL DEFAULT 'USD'"},
		{Name: "duration_unit", DDL: "`duration_unit` varchar(16) NOT NULL DEFAULT 'month'"},
		{Name: "duration_value", DDL: "`duration_value` integer NOT NULL DEFAULT 1"},
		{Name: "custom_seconds", DDL: "`custom_seconds` bigint NOT NULL DEFAULT 0"},
		{Name: "enabled", DDL: "`enabled` numeric DEFAULT 1"},
		{Name: "sort_order", DDL: "`sort_order` integer DEFAULT 0"},
		{Name: "stripe_price_id", DDL: "`stripe_price_id` varchar(128) DEFAULT ''"},
		{Name: "creem_product_id", DDL: "`creem_product_id` varchar(128) DEFAULT ''"},
		{Name: "max_purchase_per_user", DDL: "`max_purchase_per_user` integer DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "accessible_groups", DDL: "`accessible_groups` json"},
		{Name: "restricted_groups", DDL: "`restricted_groups` json"},
		{Name: "total_amount", DDL: "`total_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "weekly_amount_limit", DDL: "`weekly_amount_limit` bigint NOT NULL DEFAULT 0"},
		{Name: "special_quota_enabled", DDL: "`special_quota_enabled` numeric DEFAULT 0"},
		{Name: "hourly_reset_hours", DDL: "`hourly_reset_hours` integer DEFAULT 5"},
		{Name: "hourly_amount_limit", DDL: "`hourly_amount_limit` bigint NOT NULL DEFAULT 0"},
		{Name: "special_weekly_reset_weeks", DDL: "`special_weekly_reset_weeks` integer DEFAULT 1"},
		{Name: "special_weekly_amount_limit", DDL: "`special_weekly_amount_limit` bigint NOT NULL DEFAULT 0"},
		{Name: "special_config_updated_at", DDL: "`special_config_updated_at` bigint DEFAULT 0"},
		{Name: "quota_reset_period", DDL: "`quota_reset_period` varchar(16) DEFAULT 'never'"},
		{Name: "quota_reset_custom_seconds", DDL: "`quota_reset_custom_seconds` bigint DEFAULT 0"},
		{Name: "created_at", DDL: "`created_at` bigint"},
		{Name: "updated_at", DDL: "`updated_at` bigint"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := db.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

// migrateTokenModelLimitsToText migrates model_limits column from varchar(1024) to text
// This is safe to run multiple times - it checks the column type first
func migrateTokenModelLimitsToText(db *gorm.DB) error {
	// SQLite uses type affinity, so TEXT and VARCHAR are effectively the same — no migration needed
	if common.UsingSQLite {
		return nil
	}

	tableName := "tokens"
	columnName := "model_limits"

	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	if !db.Migrator().HasColumn(&Token{}, columnName) {
		return nil
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		var dataType string
		if err := db.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text`, tableName, columnName)
	} else if common.UsingMySQL {
		var columnType string
		if err := db.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.ToLower(columnType) == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text", tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := db.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
		}
		common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to text", tableName, columnName))
	}
	return nil
}

// migrateSubscriptionPlanPriceAmount migrates price_amount column from float/double to decimal(10,6)
// This is safe to run multiple times - it checks the column type first
func migrateSubscriptionPlanPriceAmount(db *gorm.DB) error {
	// SQLite doesn't support ALTER COLUMN, and its type affinity handles this automatically
	// Skip early to avoid GORM parsing the existing table DDL which may cause issues
	if common.UsingSQLite {
		return nil
	}

	tableName := "subscription_plans"
	columnName := "price_amount"

	// Check if table exists first
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	// Check if column exists
	if !db.Migrator().HasColumn(&SubscriptionPlan{}, columnName) {
		return nil
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		// PostgreSQL: Check if already decimal/numeric
		var dataType string
		if err := db.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			return fmt.Errorf("failed to query metadata for %s.%s: %w", tableName, columnName, err)
		} else if dataType == "numeric" {
			return nil // Already decimal/numeric
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE decimal(10,6) USING %s::decimal(10,6)`,
			tableName, columnName, columnName)
	} else if common.UsingMySQL {
		// MySQL: Check if already decimal
		var columnType string
		if err := db.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			return fmt.Errorf("failed to query metadata for %s.%s: %w", tableName, columnName, err)
		} else if strings.HasPrefix(strings.ToLower(columnType), "decimal") {
			return nil // Already decimal
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s decimal(10,6) NOT NULL DEFAULT 0",
			tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := db.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to decimal: %w", tableName, columnName, err)
		} else {
			common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to decimal(10,6)", tableName, columnName))
		}
	}
	return nil
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}
	return closeDB(DB)
}

// checkMySQLChineseSupport ensures the MySQL connection and current schema
// default charset/collation can store Chinese characters. It allows common
// Chinese-capable charsets (utf8mb4, utf8, gbk, big5, gb18030) and panics otherwise.
func checkMySQLChineseSupport(db *gorm.DB) error {
	// 仅检测：当前库默认字符集/排序规则 + 各表的排序规则（隐含字符集）

	// Read current schema defaults
	var schemaCharset, schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	// Allowed charsets that can store Chinese text
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		// 如果仅提供了排序规则，尝试按排序规则前缀判断
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	// 1) 当前库默认值必须支持中文
	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	// 2) 所有物理表的排序规则（隐含字符集）必须支持中文
	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		// NULL 或空表示继承库默认设置，已在上面校验库默认，视为通过
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		cl := *t.Collation
		// 仅凭排序规则判断是否中文可用
		ok := false
		lower := strings.ToLower(cl)
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, cl))
		}
	}

	if len(badTables) > 0 {
		// 限制输出数量以避免日志过长
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
