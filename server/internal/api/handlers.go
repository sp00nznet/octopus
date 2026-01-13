package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sp00nznet/octopus/internal/db"
	"github.com/sp00nznet/octopus/internal/providers/vmware"
	"github.com/sp00nznet/octopus/internal/sync"
)

// Authentication handlers
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, token, err := s.auth.Authenticate(creds.Username, creds.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Update or create user in database
	_, err = s.db.Exec(`
		INSERT INTO users (username, display_name, last_login)
		VALUES (?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET last_login = ?
	`, user.Username, user.DisplayName, time.Now(), time.Now())
	if err != nil {
		// Log error but don't fail login
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// Source environment handlers
func (s *Server) listSourceEnvironments(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, name, type, host, username, datacenter, cluster, created_at, updated_at
		FROM source_environments
		ORDER BY name
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var envs []db.SourceEnvironment
	for rows.Next() {
		var env db.SourceEnvironment
		err := rows.Scan(&env.ID, &env.Name, &env.Type, &env.Host, &env.Username,
			&env.Datacenter, &env.Cluster, &env.CreatedAt, &env.UpdatedAt)
		if err != nil {
			continue
		}
		envs = append(envs, env)
	}

	respondJSON(w, http.StatusOK, envs)
}

func (s *Server) createSourceEnvironment(w http.ResponseWriter, r *http.Request) {
	var env struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Host       string `json:"host"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Datacenter string `json:"datacenter"`
		Cluster    string `json:"cluster"`
	}

	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := s.db.Exec(`
		INSERT INTO source_environments (name, type, host, username, password, datacenter, cluster)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, env.Name, env.Type, env.Host, env.Username, env.Password, env.Datacenter, env.Cluster)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create environment")
		return
	}

	id, _ := result.LastInsertId()
	respondJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) getSourceEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var env db.SourceEnvironment
	err := s.db.QueryRow(`
		SELECT id, name, type, host, username, datacenter, cluster, created_at, updated_at
		FROM source_environments WHERE id = ?
	`, id).Scan(&env.ID, &env.Name, &env.Type, &env.Host, &env.Username,
		&env.Datacenter, &env.Cluster, &env.CreatedAt, &env.UpdatedAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Environment not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, env)
}

func (s *Server) updateSourceEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var env struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Host       string `json:"host"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Datacenter string `json:"datacenter"`
		Cluster    string `json:"cluster"`
	}

	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// If password is empty, don't update it
	if env.Password != "" {
		_, err := s.db.Exec(`
			UPDATE source_environments
			SET name=?, type=?, host=?, username=?, password=?, datacenter=?, cluster=?, updated_at=?
			WHERE id=?
		`, env.Name, env.Type, env.Host, env.Username, env.Password, env.Datacenter, env.Cluster, time.Now(), id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to update environment")
			return
		}
	} else {
		_, err := s.db.Exec(`
			UPDATE source_environments
			SET name=?, type=?, host=?, username=?, datacenter=?, cluster=?, updated_at=?
			WHERE id=?
		`, env.Name, env.Type, env.Host, env.Username, env.Datacenter, env.Cluster, time.Now(), id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to update environment")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteSourceEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Delete migrations associated with VMs from this source
	s.db.Exec("DELETE FROM migration_jobs WHERE source_env_id = ?", id)

	// Delete scheduled tasks for those migrations
	s.db.Exec("DELETE FROM scheduled_tasks WHERE source_env_id = ?", id)

	// Delete VMs from this source
	_, err := s.db.Exec("DELETE FROM vms WHERE source_env_id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete associated VMs")
		return
	}

	// Delete the source environment
	_, err = s.db.Exec("DELETE FROM source_environments WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete environment")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) syncSourceEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Get environment details
	var env db.SourceEnvironment
	var password string
	err := s.db.QueryRow(`
		SELECT id, name, type, host, username, password, datacenter, cluster
		FROM source_environments WHERE id = ?
	`, id).Scan(&env.ID, &env.Name, &env.Type, &env.Host, &env.Username, &password, &env.Datacenter, &env.Cluster)
	if err != nil {
		respondError(w, http.StatusNotFound, "Environment not found")
		return
	}

	// Connect to vCenter and fetch VMs
	client, err := vmware.NewClient(env.Host, env.Username, password, env.Datacenter, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to connect to vCenter: "+err.Error())
		return
	}
	defer client.Logout()

	vms, err := client.ListVMs()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list VMs: "+err.Error())
		return
	}

	// Update VMs in database
	for _, vm := range vms {
		_, err = s.db.Exec(`
			INSERT INTO vms (source_env_id, name, uuid, cpu_count, memory_mb, disk_size_gb, guest_os,
				power_state, ip_addresses, mac_addresses, port_groups, hardware_version, vmware_tools_status, last_synced)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(uuid) DO UPDATE SET
				name=?, cpu_count=?, memory_mb=?, disk_size_gb=?, guest_os=?,
				power_state=?, ip_addresses=?, mac_addresses=?, port_groups=?,
				hardware_version=?, vmware_tools_status=?, last_synced=?
		`, env.ID, vm.Name, vm.UUID, vm.CPUCount, vm.MemoryMB, vm.DiskSizeGB, vm.GuestOS,
			vm.PowerState, vm.IPAddresses, vm.MACAddresses, vm.PortGroups, vm.HardwareVersion,
			vm.VMwareToolsStatus, time.Now(),
			vm.Name, vm.CPUCount, vm.MemoryMB, vm.DiskSizeGB, vm.GuestOS,
			vm.PowerState, vm.IPAddresses, vm.MACAddresses, vm.PortGroups,
			vm.HardwareVersion, vm.VMwareToolsStatus, time.Now())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "synced",
		"vm_count": len(vms),
	})
}

// Target environment handlers
func (s *Server) listTargetEnvironments(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, name, type, config_json, created_at, updated_at
		FROM target_environments
		ORDER BY name
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var envs []db.TargetEnvironment
	for rows.Next() {
		var env db.TargetEnvironment
		err := rows.Scan(&env.ID, &env.Name, &env.Type, &env.ConfigJSON, &env.CreatedAt, &env.UpdatedAt)
		if err != nil {
			continue
		}
		envs = append(envs, env)
	}

	respondJSON(w, http.StatusOK, envs)
}

func (s *Server) createTargetEnvironment(w http.ResponseWriter, r *http.Request) {
	var env struct {
		Name       string          `json:"name"`
		Type       string          `json:"type"`
		ConfigJSON json.RawMessage `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := s.db.Exec(`
		INSERT INTO target_environments (name, type, config_json)
		VALUES (?, ?, ?)
	`, env.Name, env.Type, string(env.ConfigJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create target environment")
		return
	}

	id, _ := result.LastInsertId()
	respondJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) getTargetEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var env db.TargetEnvironment
	err := s.db.QueryRow(`
		SELECT id, name, type, config_json, created_at, updated_at
		FROM target_environments WHERE id = ?
	`, id).Scan(&env.ID, &env.Name, &env.Type, &env.ConfigJSON, &env.CreatedAt, &env.UpdatedAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Target environment not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, env)
}

func (s *Server) updateTargetEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var env struct {
		Name       string          `json:"name"`
		Type       string          `json:"type"`
		ConfigJSON json.RawMessage `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_, err := s.db.Exec(`
		UPDATE target_environments SET name=?, type=?, config_json=?, updated_at=?
		WHERE id=?
	`, env.Name, env.Type, string(env.ConfigJSON), time.Now(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update target environment")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteTargetEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Delete migrations targeting this environment
	s.db.Exec("DELETE FROM migration_jobs WHERE target_env_id = ?", id)

	// Delete the target environment
	_, err := s.db.Exec("DELETE FROM target_environments WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete target environment")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// VM handlers
func (s *Server) listVMs(w http.ResponseWriter, r *http.Request) {
	sourceID := r.URL.Query().Get("source_id")
	query := `
		SELECT id, source_env_id, name, uuid, cpu_count, memory_mb, disk_size_gb, guest_os,
			power_state, ip_addresses, mac_addresses, port_groups, hardware_version, vmware_tools_status, last_synced
		FROM vms
	`
	var rows *sql.Rows
	var err error

	if sourceID != "" {
		query += " WHERE source_env_id = ? ORDER BY name"
		rows, err = s.db.Query(query, sourceID)
	} else {
		query += " ORDER BY name"
		rows, err = s.db.Query(query)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var vms []db.VM
	for rows.Next() {
		var vm db.VM
		err := rows.Scan(&vm.ID, &vm.SourceEnvID, &vm.Name, &vm.UUID, &vm.CPUCount, &vm.MemoryMB,
			&vm.DiskSizeGB, &vm.GuestOS, &vm.PowerState, &vm.IPAddresses, &vm.MACAddresses,
			&vm.PortGroups, &vm.HardwareVersion, &vm.VMwareToolsStatus, &vm.LastSynced)
		if err != nil {
			continue
		}
		vms = append(vms, vm)
	}

	respondJSON(w, http.StatusOK, vms)
}

func (s *Server) getVM(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var vm db.VM
	err := s.db.QueryRow(`
		SELECT id, source_env_id, name, uuid, cpu_count, memory_mb, disk_size_gb, guest_os,
			power_state, ip_addresses, mac_addresses, port_groups, hardware_version, vmware_tools_status, last_synced
		FROM vms WHERE id = ?
	`, id).Scan(&vm.ID, &vm.SourceEnvID, &vm.Name, &vm.UUID, &vm.CPUCount, &vm.MemoryMB,
		&vm.DiskSizeGB, &vm.GuestOS, &vm.PowerState, &vm.IPAddresses, &vm.MACAddresses,
		&vm.PortGroups, &vm.HardwareVersion, &vm.VMwareToolsStatus, &vm.LastSynced)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "VM not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, vm)
}

func (s *Server) estimateVMSize(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		TargetType string `json:"target_type"`
		IsVXRail   bool   `json:"is_vxrail"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get VM details
	var vm db.VM
	err := s.db.QueryRow(`
		SELECT id, disk_size_gb, memory_mb, cpu_count FROM vms WHERE id = ?
	`, id).Scan(&vm.ID, &vm.DiskSizeGB, &vm.MemoryMB, &vm.CPUCount)
	if err != nil {
		respondError(w, http.StatusNotFound, "VM not found")
		return
	}

	estimation := sync.EstimateSize(vm.DiskSizeGB, float64(vm.MemoryMB)/1024, vm.CPUCount, req.TargetType, req.IsVXRail)

	// Store estimation
	_, err = s.db.Exec(`
		INSERT INTO size_estimations (vm_id, target_type, source_size_gb, estimated_size_gb, size_difference_gb, estimation_notes)
		VALUES (?, ?, ?, ?, ?, ?)
	`, vm.ID, req.TargetType, vm.DiskSizeGB, estimation.EstimatedSizeGB, estimation.SizeDifferenceGB, estimation.Notes)

	respondJSON(w, http.StatusOK, estimation)
}

// Migration job handlers
func (s *Server) listMigrations(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	query := `
		SELECT m.id, m.vm_id, m.source_env_id, m.target_env_id, m.status, m.progress,
			m.preserve_mac, m.preserve_port_groups, m.sync_interval_minutes, m.scheduled_cutover,
			m.error_message, m.created_by, m.created_at, m.started_at, m.completed_at,
			v.name as vm_name, s.name as source_name, t.name as target_name
		FROM migration_jobs m
		JOIN vms v ON m.vm_id = v.id
		JOIN source_environments s ON m.source_env_id = s.id
		JOIN target_environments t ON m.target_env_id = t.id
	`
	var rows *sql.Rows
	var err error

	if status != "" {
		query += " WHERE m.status = ? ORDER BY m.created_at DESC"
		rows, err = s.db.Query(query, status)
	} else {
		query += " ORDER BY m.created_at DESC"
		rows, err = s.db.Query(query)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	type MigrationWithNames struct {
		db.MigrationJob
		VMName     string `json:"vm_name"`
		SourceName string `json:"source_name"`
		TargetName string `json:"target_name"`
	}

	var migrations []MigrationWithNames
	for rows.Next() {
		var m MigrationWithNames
		err := rows.Scan(&m.ID, &m.VMID, &m.SourceEnvID, &m.TargetEnvID, &m.Status, &m.Progress,
			&m.PreserveMAC, &m.PreservePortGroups, &m.SyncIntervalMinutes, &m.ScheduledCutover,
			&m.ErrorMessage, &m.CreatedBy, &m.CreatedAt, &m.StartedAt, &m.CompletedAt,
			&m.VMName, &m.SourceName, &m.TargetName)
		if err != nil {
			continue
		}
		migrations = append(migrations, m)
	}

	respondJSON(w, http.StatusOK, migrations)
}

func (s *Server) createMigration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VMID                int64  `json:"vm_id"`
		SourceEnvID         int64  `json:"source_env_id"`
		TargetEnvID         int64  `json:"target_env_id"`
		PreserveMAC         bool   `json:"preserve_mac"`
		PreservePortGroups  bool   `json:"preserve_port_groups"`
		SyncIntervalMinutes int    `json:"sync_interval_minutes"`
		ScheduledCutover    string `json:"scheduled_cutover,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get user from context
	username := r.Context().Value("username").(string)

	var scheduledCutover *time.Time
	if req.ScheduledCutover != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduledCutover)
		if err == nil {
			scheduledCutover = &t
		}
	}

	if req.SyncIntervalMinutes == 0 {
		req.SyncIntervalMinutes = 60
	}

	result, err := s.db.Exec(`
		INSERT INTO migration_jobs (vm_id, source_env_id, target_env_id, preserve_mac, preserve_port_groups,
			sync_interval_minutes, scheduled_cutover, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, req.VMID, req.SourceEnvID, req.TargetEnvID, req.PreserveMAC, req.PreservePortGroups,
		req.SyncIntervalMinutes, scheduledCutover, username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create migration job")
		return
	}

	id, _ := result.LastInsertId()

	// Schedule cutover if provided
	if scheduledCutover != nil {
		s.db.Exec(`
			INSERT INTO scheduled_tasks (job_id, task_type, scheduled_time, created_by)
			VALUES (?, 'cutover', ?, ?)
		`, id, scheduledCutover, username)
	}

	respondJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) getMigration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var m db.MigrationJob
	err := s.db.QueryRow(`
		SELECT id, vm_id, source_env_id, target_env_id, status, progress, preserve_mac,
			preserve_port_groups, sync_interval_minutes, scheduled_cutover, error_message,
			created_by, created_at, started_at, completed_at
		FROM migration_jobs WHERE id = ?
	`, id).Scan(&m.ID, &m.VMID, &m.SourceEnvID, &m.TargetEnvID, &m.Status, &m.Progress,
		&m.PreserveMAC, &m.PreservePortGroups, &m.SyncIntervalMinutes, &m.ScheduledCutover,
		&m.ErrorMessage, &m.CreatedBy, &m.CreatedAt, &m.StartedAt, &m.CompletedAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Migration job not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, m)
}

func (s *Server) updateMigration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		SyncIntervalMinutes int    `json:"sync_interval_minutes"`
		ScheduledCutover    string `json:"scheduled_cutover"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var scheduledCutover *time.Time
	if req.ScheduledCutover != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduledCutover)
		if err == nil {
			scheduledCutover = &t
		}
	}

	_, err := s.db.Exec(`
		UPDATE migration_jobs SET sync_interval_minutes=?, scheduled_cutover=?
		WHERE id=?
	`, req.SyncIntervalMinutes, scheduledCutover, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update migration")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) cancelMigration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, err := s.db.Exec(`UPDATE migration_jobs SET status='cancelled' WHERE id=?`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to cancel migration")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) triggerSync(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	idInt, _ := strconv.ParseInt(id, 10, 64)

	// Update status to syncing
	_, err := s.db.Exec(`UPDATE migration_jobs SET status='syncing' WHERE id=?`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to trigger sync")
		return
	}

	// Trigger async sync
	go s.scheduler.TriggerSync(idInt)

	respondJSON(w, http.StatusOK, map[string]string{"status": "sync_started"})
}

func (s *Server) triggerCutover(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	idInt, _ := strconv.ParseInt(id, 10, 64)

	// Update status to cutting over
	_, err := s.db.Exec(`UPDATE migration_jobs SET status='cutting_over' WHERE id=?`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to trigger cutover")
		return
	}

	// Trigger async cutover
	go s.scheduler.TriggerCutover(idInt)

	respondJSON(w, http.StatusOK, map[string]string{"status": "cutover_started"})
}

// Scheduled task handlers
func (s *Server) listScheduledTasks(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, job_id, task_type, scheduled_time, status, result, created_by, created_at, executed_at
		FROM scheduled_tasks
		ORDER BY scheduled_time DESC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var tasks []db.ScheduledTask
	for rows.Next() {
		var t db.ScheduledTask
		err := rows.Scan(&t.ID, &t.JobID, &t.TaskType, &t.ScheduledTime, &t.Status,
			&t.Result, &t.CreatedBy, &t.CreatedAt, &t.ExecutedAt)
		if err != nil {
			continue
		}
		tasks = append(tasks, t)
	}

	respondJSON(w, http.StatusOK, tasks)
}

func (s *Server) createScheduledTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JobID         int64  `json:"job_id"`
		TaskType      string `json:"task_type"`
		ScheduledTime string `json:"scheduled_time"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	scheduledTime, err := time.Parse(time.RFC3339, req.ScheduledTime)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid scheduled time format")
		return
	}

	username := r.Context().Value("username").(string)

	result, err := s.db.Exec(`
		INSERT INTO scheduled_tasks (job_id, task_type, scheduled_time, created_by)
		VALUES (?, ?, ?, ?)
	`, req.JobID, req.TaskType, scheduledTime, username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create scheduled task")
		return
	}

	id, _ := result.LastInsertId()
	respondJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) getScheduledTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var t db.ScheduledTask
	err := s.db.QueryRow(`
		SELECT id, job_id, task_type, scheduled_time, status, result, created_by, created_at, executed_at
		FROM scheduled_tasks WHERE id = ?
	`, id).Scan(&t.ID, &t.JobID, &t.TaskType, &t.ScheduledTime, &t.Status,
		&t.Result, &t.CreatedBy, &t.CreatedAt, &t.ExecutedAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Scheduled task not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, t)
}

func (s *Server) cancelScheduledTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, err := s.db.Exec(`UPDATE scheduled_tasks SET status='cancelled' WHERE id=?`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to cancel task")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// Unified Environment handlers
func (s *Server) listEnvironments(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, name, type, config_json, created_at, updated_at
		FROM environments
		ORDER BY name
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var envs []db.Environment
	for rows.Next() {
		var env db.Environment
		err := rows.Scan(&env.ID, &env.Name, &env.Type, &env.ConfigJSON, &env.CreatedAt, &env.UpdatedAt)
		if err != nil {
			continue
		}
		envs = append(envs, env)
	}

	respondJSON(w, http.StatusOK, envs)
}

func (s *Server) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var env struct {
		Name       string          `json:"name"`
		Type       string          `json:"type"`
		ConfigJSON json.RawMessage `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := s.db.Exec(`
		INSERT INTO environments (name, type, config_json)
		VALUES (?, ?, ?)
	`, env.Name, env.Type, string(env.ConfigJSON))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create environment: "+err.Error())
		return
	}

	id, _ := result.LastInsertId()
	respondJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) getEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var env db.Environment
	err := s.db.QueryRow(`
		SELECT id, name, type, config_json, created_at, updated_at
		FROM environments WHERE id = ?
	`, id).Scan(&env.ID, &env.Name, &env.Type, &env.ConfigJSON, &env.CreatedAt, &env.UpdatedAt)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Environment not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, env)
}

func (s *Server) updateEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var env struct {
		Name       string          `json:"name"`
		Type       string          `json:"type"`
		ConfigJSON json.RawMessage `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_, err := s.db.Exec(`
		UPDATE environments SET name=?, type=?, config_json=?, updated_at=?
		WHERE id=?
	`, env.Name, env.Type, string(env.ConfigJSON), time.Now(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update environment")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, err := s.db.Exec("DELETE FROM environments WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete environment")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) syncEnvironment(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Get environment details
	var env db.Environment
	err := s.db.QueryRow(`
		SELECT id, name, type, config_json FROM environments WHERE id = ?
	`, id).Scan(&env.ID, &env.Name, &env.Type, &env.ConfigJSON)
	if err != nil {
		respondError(w, http.StatusNotFound, "Environment not found")
		return
	}

	// Only VMware environments can be synced
	if env.Type != "vmware" && env.Type != "vmware-vxrail" {
		respondError(w, http.StatusBadRequest, "Only VMware environments can be synced")
		return
	}

	// Parse config
	var config struct {
		Host       string `json:"host"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Datacenter string `json:"datacenter"`
	}
	if err := json.Unmarshal([]byte(env.ConfigJSON), &config); err != nil {
		respondError(w, http.StatusInternalServerError, "Invalid environment config")
		return
	}

	// Connect to vCenter and fetch VMs
	client, err := vmware.NewClient(config.Host, config.Username, config.Password, config.Datacenter, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to connect to vCenter: "+err.Error())
		return
	}
	defer client.Logout()

	vms, err := client.ListVMs()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list VMs: "+err.Error())
		return
	}

	// Update VMs in database - use environment ID as source_env_id
	for _, vm := range vms {
		_, err = s.db.Exec(`
			INSERT INTO vms (source_env_id, name, uuid, cpu_count, memory_mb, disk_size_gb, guest_os,
				power_state, ip_addresses, mac_addresses, port_groups, hardware_version, vmware_tools_status, last_synced)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(uuid) DO UPDATE SET
				name=?, cpu_count=?, memory_mb=?, disk_size_gb=?, guest_os=?,
				power_state=?, ip_addresses=?, mac_addresses=?, port_groups=?,
				hardware_version=?, vmware_tools_status=?, last_synced=?
		`, env.ID, vm.Name, vm.UUID, vm.CPUCount, vm.MemoryMB, vm.DiskSizeGB, vm.GuestOS,
			vm.PowerState, vm.IPAddresses, vm.MACAddresses, vm.PortGroups, vm.HardwareVersion,
			vm.VMwareToolsStatus, time.Now(),
			vm.Name, vm.CPUCount, vm.MemoryMB, vm.DiskSizeGB, vm.GuestOS,
			vm.PowerState, vm.IPAddresses, vm.MACAddresses, vm.PortGroups,
			vm.HardwareVersion, vm.VMwareToolsStatus, time.Now())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "synced",
		"vm_count": len(vms),
	})
}
