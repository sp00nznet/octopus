package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database wraps the SQL database connection
type Database struct {
	*sql.DB
}

// Initialize creates and opens the database connection
func Initialize(path string) (*Database, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Database{db}, nil
}

// RunMigrations applies all database migrations
func RunMigrations(db *Database) error {
	migrations := []string{
		// Users table for local user cache
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT,
			display_name TEXT,
			is_admin BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP
		)`,

		// Environment variables storage (admin portal)
		`CREATE TABLE IF NOT EXISTS env_variables (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			value TEXT NOT NULL,
			description TEXT,
			is_secret BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Source environments (vCenter clusters, etc.)
		`CREATE TABLE IF NOT EXISTS source_environments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			host TEXT NOT NULL,
			username TEXT,
			password TEXT,
			datacenter TEXT,
			cluster TEXT,
			config_json TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Target environments (vCenter, AWS, GCP, Azure)
		`CREATE TABLE IF NOT EXISTS target_environments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('vmware', 'aws', 'gcp', 'azure')),
			config_json TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// VMs inventory
		`CREATE TABLE IF NOT EXISTS vms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_env_id INTEGER REFERENCES source_environments(id),
			name TEXT NOT NULL,
			uuid TEXT UNIQUE,
			cpu_count INTEGER,
			memory_mb INTEGER,
			disk_size_gb REAL,
			guest_os TEXT,
			power_state TEXT,
			ip_addresses TEXT,
			mac_addresses TEXT,
			port_groups TEXT,
			hardware_version TEXT,
			vmware_tools_status TEXT,
			last_synced TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Migration jobs
		`CREATE TABLE IF NOT EXISTS migration_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vm_id INTEGER REFERENCES vms(id),
			source_env_id INTEGER REFERENCES source_environments(id),
			target_env_id INTEGER REFERENCES target_environments(id),
			status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'syncing', 'ready', 'cutting_over', 'completed', 'failed', 'cancelled')),
			progress INTEGER DEFAULT 0,
			preserve_mac BOOLEAN DEFAULT TRUE,
			preserve_port_groups BOOLEAN DEFAULT TRUE,
			sync_interval_minutes INTEGER DEFAULT 60,
			scheduled_cutover TIMESTAMP,
			error_message TEXT,
			created_by TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			started_at TIMESTAMP,
			completed_at TIMESTAMP
		)`,

		// Sync history for each migration job
		`CREATE TABLE IF NOT EXISTS sync_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER REFERENCES migration_jobs(id),
			status TEXT CHECK(status IN ('started', 'completed', 'failed')),
			bytes_transferred INTEGER,
			duration_seconds INTEGER,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Activity logs
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER REFERENCES users(id),
			action TEXT NOT NULL,
			entity_type TEXT,
			entity_id INTEGER,
			details TEXT,
			ip_address TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Size estimations
		`CREATE TABLE IF NOT EXISTS size_estimations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vm_id INTEGER REFERENCES vms(id),
			target_type TEXT NOT NULL,
			source_size_gb REAL,
			estimated_size_gb REAL,
			size_difference_gb REAL,
			estimation_notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Scheduled tasks (cutover/failover)
		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER REFERENCES migration_jobs(id),
			task_type TEXT NOT NULL CHECK(task_type IN ('cutover', 'failover', 'sync', 'test_failover')),
			scheduled_time TIMESTAMP NOT NULL,
			status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
			result TEXT,
			created_by TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			executed_at TIMESTAMP
		)`,

		// Create indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_vms_source_env ON vms(source_env_id)`,
		`CREATE INDEX IF NOT EXISTS idx_migration_jobs_status ON migration_jobs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_user ON activity_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_time ON scheduled_tasks(scheduled_time)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return err
		}
	}

	return nil
}

// VM represents a virtual machine
type VM struct {
	ID                int64     `json:"id"`
	SourceEnvID       int64     `json:"source_env_id"`
	Name              string    `json:"name"`
	UUID              string    `json:"uuid"`
	CPUCount          int       `json:"cpu_count"`
	MemoryMB          int       `json:"memory_mb"`
	DiskSizeGB        float64   `json:"disk_size_gb"`
	GuestOS           string    `json:"guest_os"`
	PowerState        string    `json:"power_state"`
	IPAddresses       string    `json:"ip_addresses"`
	MACAddresses      string    `json:"mac_addresses"`
	PortGroups        string    `json:"port_groups"`
	HardwareVersion   string    `json:"hardware_version"`
	VMwareToolsStatus string    `json:"vmware_tools_status"`
	LastSynced        time.Time `json:"last_synced"`
	CreatedAt         time.Time `json:"created_at"`
}

// MigrationJob represents a VM migration job
type MigrationJob struct {
	ID                  int64      `json:"id"`
	VMID                int64      `json:"vm_id"`
	SourceEnvID         int64      `json:"source_env_id"`
	TargetEnvID         int64      `json:"target_env_id"`
	Status              string     `json:"status"`
	Progress            int        `json:"progress"`
	PreserveMAC         bool       `json:"preserve_mac"`
	PreservePortGroups  bool       `json:"preserve_port_groups"`
	SyncIntervalMinutes int        `json:"sync_interval_minutes"`
	ScheduledCutover    *time.Time `json:"scheduled_cutover,omitempty"`
	ErrorMessage        string     `json:"error_message,omitempty"`
	CreatedBy           string     `json:"created_by"`
	CreatedAt           time.Time  `json:"created_at"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// SourceEnvironment represents a source vCenter cluster
type SourceEnvironment struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Host       string    `json:"host"`
	Username   string    `json:"username,omitempty"`
	Password   string    `json:"-"`
	Datacenter string    `json:"datacenter"`
	Cluster    string    `json:"cluster"`
	ConfigJSON string    `json:"config_json,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TargetEnvironment represents a migration target
type TargetEnvironment struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	ConfigJSON string    `json:"config_json"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ScheduledTask represents a scheduled cutover/failover
type ScheduledTask struct {
	ID            int64      `json:"id"`
	JobID         int64      `json:"job_id"`
	TaskType      string     `json:"task_type"`
	ScheduledTime time.Time  `json:"scheduled_time"`
	Status        string     `json:"status"`
	Result        string     `json:"result,omitempty"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	ExecutedAt    *time.Time `json:"executed_at,omitempty"`
}

// SizeEstimation represents a VM size estimation for a target
type SizeEstimation struct {
	ID               int64     `json:"id"`
	VMID             int64     `json:"vm_id"`
	TargetType       string    `json:"target_type"`
	SourceSizeGB     float64   `json:"source_size_gb"`
	EstimatedSizeGB  float64   `json:"estimated_size_gb"`
	SizeDifferenceGB float64   `json:"size_difference_gb"`
	EstimationNotes  string    `json:"estimation_notes"`
	CreatedAt        time.Time `json:"created_at"`
}

// EnvVariable represents an environment variable
type EnvVariable struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Value       string    `json:"value,omitempty"`
	Description string    `json:"description"`
	IsSecret    bool      `json:"is_secret"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ActivityLog represents an activity log entry
type ActivityLog struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	Details    string    `json:"details"`
	IPAddress  string    `json:"ip_address"`
	CreatedAt  time.Time `json:"created_at"`
}
