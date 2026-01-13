package scheduler

import (
	"database/sql"
	"log"
	"time"

	"github.com/sp00nznet/octopus/internal/db"
	"github.com/sp00nznet/octopus/internal/sync"
)

// Scheduler manages scheduled tasks for migrations
type Scheduler struct {
	db       *db.Database
	stopChan chan struct{}
}

// New creates a new scheduler
func New(database *db.Database) *Scheduler {
	return &Scheduler{
		db:       database,
		stopChan: make(chan struct{}),
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start() {
	log.Println("Scheduler started")

	// Check for due tasks every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Also run sync tasks based on their intervals
	syncTicker := time.NewTicker(5 * time.Minute)
	defer syncTicker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processDueTasks()
		case <-syncTicker.C:
			s.processSyncJobs()
		case <-s.stopChan:
			log.Println("Scheduler stopped")
			return
		}
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// processDueTasks finds and executes scheduled tasks that are due
func (s *Scheduler) processDueTasks() {
	rows, err := s.db.Query(`
		SELECT id, job_id, task_type, scheduled_time
		FROM scheduled_tasks
		WHERE status = 'pending' AND scheduled_time <= ?
	`, time.Now())
	if err != nil {
		log.Printf("Error fetching due tasks: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var task struct {
			ID            int64
			JobID         int64
			TaskType      string
			ScheduledTime time.Time
		}

		if err := rows.Scan(&task.ID, &task.JobID, &task.TaskType, &task.ScheduledTime); err != nil {
			continue
		}

		// Mark as running
		s.db.Exec(`UPDATE scheduled_tasks SET status = 'running' WHERE id = ?`, task.ID)

		// Execute task
		go s.executeTask(task.ID, task.JobID, task.TaskType)
	}
}

// processSyncJobs finds migration jobs that need syncing
func (s *Scheduler) processSyncJobs() {
	rows, err := s.db.Query(`
		SELECT m.id, m.vm_id, m.source_env_id, m.target_env_id, m.sync_interval_minutes,
			m.preserve_mac, m.preserve_port_groups, v.name as vm_name,
			COALESCE(MAX(sh.created_at), m.created_at) as last_sync
		FROM migration_jobs m
		JOIN vms v ON m.vm_id = v.id
		LEFT JOIN sync_history sh ON m.id = sh.job_id AND sh.status = 'completed'
		WHERE m.status IN ('syncing', 'ready')
		GROUP BY m.id
	`)
	if err != nil {
		log.Printf("Error fetching sync jobs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var job struct {
			ID                  int64
			VMID                int64
			SourceEnvID         int64
			TargetEnvID         int64
			SyncIntervalMinutes int
			PreserveMAC         bool
			PreservePortGroups  bool
			VMName              string
			LastSync            time.Time
		}

		if err := rows.Scan(&job.ID, &job.VMID, &job.SourceEnvID, &job.TargetEnvID,
			&job.SyncIntervalMinutes, &job.PreserveMAC, &job.PreservePortGroups,
			&job.VMName, &job.LastSync); err != nil {
			continue
		}

		// Check if sync is due
		nextSync := job.LastSync.Add(time.Duration(job.SyncIntervalMinutes) * time.Minute)
		if time.Now().After(nextSync) {
			go s.performSync(job.ID, job.VMName, job.PreserveMAC, job.PreservePortGroups)
		}
	}
}

// executeTask executes a scheduled task
func (s *Scheduler) executeTask(taskID, jobID int64, taskType string) {
	startTime := time.Now()
	var result string
	var err error

	switch taskType {
	case "cutover":
		err = s.TriggerCutover(jobID)
	case "failover":
		err = s.TriggerCutover(jobID) // Failover uses same logic
	case "sync":
		s.TriggerSync(jobID)
	case "test_failover":
		err = s.performTestFailover(jobID)
	}

	status := "completed"
	if err != nil {
		status = "failed"
		result = err.Error()
	}

	s.db.Exec(`
		UPDATE scheduled_tasks
		SET status = ?, result = ?, executed_at = ?
		WHERE id = ?
	`, status, result, startTime, taskID)
}

// TriggerSync triggers a sync operation for a migration job
func (s *Scheduler) TriggerSync(jobID int64) {
	// Get job details
	var job struct {
		ID                 int64
		VMID               int64
		SourceEnvID        int64
		TargetEnvID        int64
		PreserveMAC        bool
		PreservePortGroups bool
		VMName             string
		SourceType         string
		TargetType         string
	}

	err := s.db.QueryRow(`
		SELECT m.id, m.vm_id, m.source_env_id, m.target_env_id,
			m.preserve_mac, m.preserve_port_groups, v.name,
			s.type as source_type, t.type as target_type
		FROM migration_jobs m
		JOIN vms v ON m.vm_id = v.id
		JOIN source_environments s ON m.source_env_id = s.id
		JOIN target_environments t ON m.target_env_id = t.id
		WHERE m.id = ?
	`, jobID).Scan(&job.ID, &job.VMID, &job.SourceEnvID, &job.TargetEnvID,
		&job.PreserveMAC, &job.PreservePortGroups, &job.VMName,
		&job.SourceType, &job.TargetType)
	if err != nil {
		log.Printf("Error getting job %d: %v", jobID, err)
		return
	}

	s.performSync(jobID, job.VMName, job.PreserveMAC, job.PreservePortGroups)
}

// performSync performs the actual sync operation
func (s *Scheduler) performSync(jobID int64, vmName string, preserveMAC, preservePortGroups bool) {
	startTime := time.Now()

	// Record sync start
	s.db.Exec(`
		INSERT INTO sync_history (job_id, status)
		VALUES (?, 'started')
	`, jobID)

	// Update job status
	s.db.Exec(`UPDATE migration_jobs SET status = 'syncing' WHERE id = ?`, jobID)

	// Get source and target configs
	var sourceType, targetType string
	var sourceConfig, targetConfig sql.NullString

	s.db.QueryRow(`
		SELECT s.type, s.config_json, t.type, t.config_json
		FROM migration_jobs m
		JOIN source_environments s ON m.source_env_id = s.id
		JOIN target_environments t ON m.target_env_id = t.id
		WHERE m.id = ?
	`, jobID).Scan(&sourceType, &sourceConfig, &targetType, &targetConfig)

	// Create sync manager and perform sync
	syncMgr := sync.NewSyncManager(jobID, sourceType, targetType, nil, nil)
	result, err := syncMgr.PerformSync(vmName, preserveMAC, preservePortGroups)

	// Record result
	status := "completed"
	errorMsg := ""
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}

	duration := int(time.Since(startTime).Seconds())
	bytesTransferred := int64(0)
	if result != nil {
		bytesTransferred = result.BytesTransferred
	}

	s.db.Exec(`
		INSERT INTO sync_history (job_id, status, bytes_transferred, duration_seconds, error_message)
		VALUES (?, ?, ?, ?, ?)
	`, jobID, status, bytesTransferred, duration, errorMsg)

	// Update job status
	if err != nil {
		s.db.Exec(`UPDATE migration_jobs SET status = 'failed', error_message = ? WHERE id = ?`, errorMsg, jobID)
	} else {
		s.db.Exec(`UPDATE migration_jobs SET status = 'ready' WHERE id = ?`, jobID)
	}
}

// TriggerCutover triggers a cutover operation for a migration job
func (s *Scheduler) TriggerCutover(jobID int64) error {
	// Get job details
	var vmName, sourceType, targetType string
	err := s.db.QueryRow(`
		SELECT v.name, s.type, t.type
		FROM migration_jobs m
		JOIN vms v ON m.vm_id = v.id
		JOIN source_environments s ON m.source_env_id = s.id
		JOIN target_environments t ON m.target_env_id = t.id
		WHERE m.id = ?
	`, jobID).Scan(&vmName, &sourceType, &targetType)
	if err != nil {
		return err
	}

	// Update status
	s.db.Exec(`UPDATE migration_jobs SET status = 'cutting_over', started_at = ? WHERE id = ?`, time.Now(), jobID)

	// Perform cutover
	syncMgr := sync.NewSyncManager(jobID, sourceType, targetType, nil, nil)
	err = syncMgr.PerformCutover(vmName)

	if err != nil {
		s.db.Exec(`UPDATE migration_jobs SET status = 'failed', error_message = ? WHERE id = ?`, err.Error(), jobID)
		return err
	}

	// Mark as completed
	s.db.Exec(`UPDATE migration_jobs SET status = 'completed', completed_at = ?, progress = 100 WHERE id = ?`, time.Now(), jobID)

	return nil
}

// performTestFailover performs a test failover (non-destructive)
func (s *Scheduler) performTestFailover(jobID int64) error {
	// A test failover creates a temporary copy of the VM at the target
	// without affecting the source VM

	var vmName, targetType string
	err := s.db.QueryRow(`
		SELECT v.name, t.type
		FROM migration_jobs m
		JOIN vms v ON m.vm_id = v.id
		JOIN target_environments t ON m.target_env_id = t.id
		WHERE m.id = ?
	`, jobID).Scan(&vmName, &targetType)
	if err != nil {
		return err
	}

	log.Printf("Performing test failover for VM %s to %s", vmName, targetType)

	// Create a test VM at the target with a modified name
	testVMName := vmName + "-test-failover"
	_ = testVMName // Would be used to create test VM

	return nil
}

// GetSyncHistory returns sync history for a job
func (s *Scheduler) GetSyncHistory(jobID int64) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT id, status, bytes_transferred, duration_seconds, error_message, created_at
		FROM sync_history
		WHERE job_id = ?
		ORDER BY created_at DESC
		LIMIT 50
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var h struct {
			ID               int64
			Status           string
			BytesTransferred int64
			DurationSeconds  int
			ErrorMessage     sql.NullString
			CreatedAt        time.Time
		}

		if err := rows.Scan(&h.ID, &h.Status, &h.BytesTransferred, &h.DurationSeconds,
			&h.ErrorMessage, &h.CreatedAt); err != nil {
			continue
		}

		entry := map[string]interface{}{
			"id":                h.ID,
			"status":            h.Status,
			"bytes_transferred": h.BytesTransferred,
			"duration_seconds":  h.DurationSeconds,
			"created_at":        h.CreatedAt,
		}
		if h.ErrorMessage.Valid {
			entry["error_message"] = h.ErrorMessage.String
		}
		history = append(history, entry)
	}

	return history, nil
}
