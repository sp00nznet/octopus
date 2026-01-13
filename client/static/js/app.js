// Octopus Web Application

// Check authentication
function checkAuth() {
    const token = localStorage.getItem('octopus_token');
    if (!token) {
        window.location.href = '/login';
        return false;
    }
    return true;
}

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    if (!checkAuth()) return;

    // Load user info
    const user = JSON.parse(localStorage.getItem('octopus_user') || '{}');
    document.getElementById('current-user').textContent = user.display_name || user.username || 'User';

    // Show admin elements if user is admin
    if (user.is_admin) {
        document.body.classList.add('is-admin');
    }

    // Set up navigation
    setupNavigation();

    // Load initial data
    loadDashboard();

    // Set up logout
    document.getElementById('logout-btn').addEventListener('click', logout);

    // Auto-refresh data
    setInterval(refreshCurrentPage, 30000);
});

// Navigation
function setupNavigation() {
    const navLinks = document.querySelectorAll('.nav-link');
    navLinks.forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const page = link.dataset.page;
            navigateTo(page);
        });
    });

    // Handle browser back/forward
    window.addEventListener('popstate', () => {
        const page = window.location.hash.replace('#', '') || 'dashboard';
        showPage(page);
    });

    // Show initial page based on hash
    const initialPage = window.location.hash.replace('#', '') || 'dashboard';
    showPage(initialPage);
}

function navigateTo(page) {
    window.location.hash = page;
    showPage(page);
}

function showPage(page) {
    // Update nav
    document.querySelectorAll('.nav-link').forEach(link => {
        link.classList.toggle('active', link.dataset.page === page);
    });

    // Show page
    document.querySelectorAll('.page').forEach(p => {
        p.classList.remove('active');
    });
    const pageEl = document.getElementById(`${page}-page`);
    if (pageEl) {
        pageEl.classList.add('active');
        loadPageData(page);
    }
}

function loadPageData(page) {
    switch (page) {
        case 'dashboard':
            loadDashboard();
            break;
        case 'sources':
            loadSources();
            break;
        case 'targets':
            loadTargets();
            break;
        case 'vms':
            loadVMs();
            break;
        case 'migrations':
            loadMigrations();
            break;
        case 'schedules':
            loadSchedules();
            break;
        case 'admin':
            loadAdminData();
            break;
    }
}

function refreshCurrentPage() {
    const activePage = document.querySelector('.page.active');
    if (activePage) {
        const page = activePage.id.replace('-page', '');
        loadPageData(page);
    }
}

// Logout
function logout() {
    api.clearToken();
    window.location.href = '/login';
}

// Dashboard
async function loadDashboard() {
    try {
        const [sources, targets, vms, migrations, schedules] = await Promise.all([
            api.getSources(),
            api.getTargets(),
            api.getVMs(),
            api.getMigrations(),
            api.getScheduledTasks(),
        ]);

        document.getElementById('stat-sources').textContent = sources?.length || 0;
        document.getElementById('stat-targets').textContent = targets?.length || 0;
        document.getElementById('stat-vms').textContent = vms?.length || 0;
        document.getElementById('stat-migrations').textContent =
            migrations?.filter(m => ['syncing', 'ready', 'cutting_over'].includes(m.status)).length || 0;

        // Recent migrations
        const recentMigrations = (migrations || []).slice(0, 5);
        const migrationsBody = document.querySelector('#recent-migrations-table tbody');
        migrationsBody.innerHTML = recentMigrations.map(m => `
            <tr>
                <td>${m.vm_name}</td>
                <td>${m.source_name}</td>
                <td>${m.target_name}</td>
                <td><span class="status-badge status-${m.status}">${m.status}</span></td>
                <td>
                    <div class="progress-bar">
                        <div class="progress-bar-fill" style="width: ${m.progress}%"></div>
                    </div>
                </td>
            </tr>
        `).join('') || '<tr><td colspan="5">No migrations</td></tr>';

        // Upcoming tasks
        const upcomingTasks = (schedules || [])
            .filter(t => t.status === 'pending')
            .slice(0, 5);
        const tasksBody = document.querySelector('#upcoming-tasks-table tbody');
        tasksBody.innerHTML = upcomingTasks.map(t => `
            <tr>
                <td>${t.task_type}</td>
                <td>${t.job_id}</td>
                <td>${new Date(t.scheduled_time).toLocaleString()}</td>
                <td><span class="status-badge status-${t.status}">${t.status}</span></td>
            </tr>
        `).join('') || '<tr><td colspan="4">No scheduled tasks</td></tr>';

    } catch (error) {
        console.error('Failed to load dashboard:', error);
    }
}

// Sources
async function loadSources() {
    try {
        const sources = await api.getSources();
        const tbody = document.querySelector('#sources-table tbody');
        tbody.innerHTML = (sources || []).map(s => `
            <tr>
                <td>${s.name}</td>
                <td>${s.type}</td>
                <td>${s.host}</td>
                <td>${s.datacenter || '-'}</td>
                <td class="action-buttons">
                    <button class="btn btn-small btn-secondary" onclick="syncSource(${s.id})">Sync</button>
                    <button class="btn btn-small btn-danger" onclick="deleteSource(${s.id})">Delete</button>
                </td>
            </tr>
        `).join('') || '<tr><td colspan="5">No source environments</td></tr>';

        // Update VM filter
        const filter = document.getElementById('vm-source-filter');
        filter.innerHTML = '<option value="">All Sources</option>' +
            (sources || []).map(s => `<option value="${s.id}">${s.name}</option>`).join('');

    } catch (error) {
        console.error('Failed to load sources:', error);
    }
}

function showAddSourceModal() {
    document.getElementById('add-source-form').reset();
    openModal('add-source-modal');
}

async function addSource(event) {
    event.preventDefault();
    try {
        await api.createSource({
            name: document.getElementById('source-name').value,
            type: document.getElementById('source-type').value,
            host: document.getElementById('source-host').value,
            username: document.getElementById('source-username').value,
            password: document.getElementById('source-password').value,
            datacenter: document.getElementById('source-datacenter').value,
            cluster: document.getElementById('source-cluster').value,
        });
        closeModal();
        loadSources();
    } catch (error) {
        alert('Failed to add source: ' + error.message);
    }
}

async function syncSource(id) {
    try {
        const result = await api.syncSource(id);
        alert(`Synced ${result.vm_count} VMs`);
        loadVMs();
    } catch (error) {
        alert('Failed to sync: ' + error.message);
    }
}

async function deleteSource(id) {
    if (!confirm('Are you sure you want to delete this source environment?')) return;
    try {
        await api.deleteSource(id);
        loadSources();
    } catch (error) {
        alert('Failed to delete: ' + error.message);
    }
}

async function syncAllSources() {
    const sources = await api.getSources();
    for (const source of sources || []) {
        try {
            await api.syncSource(source.id);
        } catch (error) {
            console.error(`Failed to sync ${source.name}:`, error);
        }
    }
    loadVMs();
}

// Targets
async function loadTargets() {
    try {
        const targets = await api.getTargets();
        const tbody = document.querySelector('#targets-table tbody');
        tbody.innerHTML = (targets || []).map(t => `
            <tr>
                <td>${t.name}</td>
                <td>${t.type}</td>
                <td>${new Date(t.created_at).toLocaleDateString()}</td>
                <td class="action-buttons">
                    <button class="btn btn-small btn-danger" onclick="deleteTarget(${t.id})">Delete</button>
                </td>
            </tr>
        `).join('') || '<tr><td colspan="4">No target environments</td></tr>';
    } catch (error) {
        console.error('Failed to load targets:', error);
    }
}

function showAddTargetModal() {
    document.getElementById('add-target-form').reset();
    updateTargetConfigFields();
    openModal('add-target-modal');
}

function updateTargetConfigFields() {
    const type = document.getElementById('target-type').value;
    const container = document.getElementById('target-config-fields');

    const fields = {
        vmware: `
            <div class="form-group">
                <label>Host</label>
                <input type="text" name="host" required>
            </div>
            <div class="form-group">
                <label>Username</label>
                <input type="text" name="username" required>
            </div>
            <div class="form-group">
                <label>Password</label>
                <input type="password" name="password" required>
            </div>
            <div class="form-group">
                <label>Datacenter</label>
                <input type="text" name="datacenter" required>
            </div>
        `,
        aws: `
            <div class="form-group">
                <label>Region</label>
                <input type="text" name="region" placeholder="us-east-1" required>
            </div>
            <div class="form-group">
                <label>Access Key ID</label>
                <input type="text" name="access_key_id" required>
            </div>
            <div class="form-group">
                <label>Secret Access Key</label>
                <input type="password" name="secret_access_key" required>
            </div>
        `,
        gcp: `
            <div class="form-group">
                <label>Project ID</label>
                <input type="text" name="project_id" required>
            </div>
            <div class="form-group">
                <label>Zone</label>
                <input type="text" name="zone" placeholder="us-central1-a" required>
            </div>
            <div class="form-group">
                <label>Service Account JSON</label>
                <textarea name="credentials" required></textarea>
            </div>
        `,
        azure: `
            <div class="form-group">
                <label>Subscription ID</label>
                <input type="text" name="subscription_id" required>
            </div>
            <div class="form-group">
                <label>Resource Group</label>
                <input type="text" name="resource_group" required>
            </div>
            <div class="form-group">
                <label>Tenant ID</label>
                <input type="text" name="tenant_id" required>
            </div>
            <div class="form-group">
                <label>Client ID</label>
                <input type="text" name="client_id" required>
            </div>
            <div class="form-group">
                <label>Client Secret</label>
                <input type="password" name="client_secret" required>
            </div>
        `,
    };

    container.innerHTML = fields[type] || '';
}

async function addTarget(event) {
    event.preventDefault();
    const form = event.target;
    const type = document.getElementById('target-type').value;

    // Collect config fields
    const config = {};
    const inputs = document.querySelectorAll('#target-config-fields input, #target-config-fields textarea');
    inputs.forEach(input => {
        config[input.name] = input.value;
    });

    try {
        await api.createTarget({
            name: document.getElementById('target-name').value,
            type: type,
            config: config,
        });
        closeModal();
        loadTargets();
    } catch (error) {
        alert('Failed to add target: ' + error.message);
    }
}

async function deleteTarget(id) {
    if (!confirm('Are you sure you want to delete this target environment?')) return;
    try {
        await api.deleteTarget(id);
        loadTargets();
    } catch (error) {
        alert('Failed to delete: ' + error.message);
    }
}

// VMs
async function loadVMs() {
    try {
        const sourceId = document.getElementById('vm-source-filter').value;
        const vms = await api.getVMs(sourceId || null);
        const tbody = document.querySelector('#vms-table tbody');
        tbody.innerHTML = (vms || []).map(vm => `
            <tr>
                <td>${vm.name}</td>
                <td>${vm.cpu_count}</td>
                <td>${(vm.memory_mb / 1024).toFixed(1)}</td>
                <td>${vm.disk_size_gb.toFixed(1)}</td>
                <td><span class="status-badge status-${vm.power_state === 'poweredOn' ? 'ready' : 'pending'}">${vm.power_state}</span></td>
                <td>${vm.last_synced ? new Date(vm.last_synced).toLocaleString() : '-'}</td>
                <td class="action-buttons">
                    <button class="btn btn-small btn-secondary" onclick="showSizeEstimation(${vm.id})">Estimate Size</button>
                    <button class="btn btn-small btn-primary" onclick="startMigrationForVM(${vm.id})">Migrate</button>
                </td>
            </tr>
        `).join('') || '<tr><td colspan="7">No VMs found. Sync a source environment first.</td></tr>';
    } catch (error) {
        console.error('Failed to load VMs:', error);
    }
}

async function showSizeEstimation(vmId) {
    const content = document.getElementById('size-estimation-content');
    content.innerHTML = '<p>Loading...</p>';
    openModal('size-estimation-modal');

    try {
        const results = await Promise.all([
            api.estimateVMSize(vmId, 'vmware', false),
            api.estimateVMSize(vmId, 'aws', false),
            api.estimateVMSize(vmId, 'gcp', false),
            api.estimateVMSize(vmId, 'azure', false),
        ]);

        content.innerHTML = `
            <div class="size-estimation-result">
                <h4>Size Estimations by Target</h4>
                ${results.map((r, i) => {
                    const targets = ['VMware', 'AWS', 'GCP', 'Azure'];
                    return `
                        <div class="target-estimation">
                            <h5>${targets[i]}</h5>
                            <div class="stat">
                                <span class="stat-label">Source Size</span>
                                <span class="stat-value">${r.source_size_gb.toFixed(2)} GB</span>
                            </div>
                            <div class="stat">
                                <span class="stat-label">Estimated Size</span>
                                <span class="stat-value">${r.estimated_size_gb.toFixed(2)} GB</span>
                            </div>
                            <div class="stat">
                                <span class="stat-label">Difference</span>
                                <span class="stat-value">${r.size_difference_gb > 0 ? '+' : ''}${r.size_difference_gb.toFixed(2)} GB</span>
                            </div>
                            <div class="notes">${r.notes}</div>
                        </div>
                        <hr>
                    `;
                }).join('')}
            </div>
        `;
    } catch (error) {
        content.innerHTML = `<p class="error">Failed to estimate size: ${error.message}</p>`;
    }
}

async function startMigrationForVM(vmId) {
    // Get VM info to pre-fill form
    const vm = await api.getVM(vmId);
    document.getElementById('migration-vm').value = vmId;
    document.getElementById('migration-source').value = vm.source_env_id;
    showCreateMigrationModal();
}

// Migrations
async function loadMigrations() {
    try {
        const status = document.getElementById('migration-status-filter').value;
        const migrations = await api.getMigrations(status || null);
        const tbody = document.querySelector('#migrations-table tbody');
        tbody.innerHTML = (migrations || []).map(m => `
            <tr>
                <td>${m.vm_name}</td>
                <td>${m.source_name}</td>
                <td>${m.target_name}</td>
                <td><span class="status-badge status-${m.status}">${m.status}</span></td>
                <td>
                    <div class="progress-bar">
                        <div class="progress-bar-fill" style="width: ${m.progress}%"></div>
                    </div>
                    <small>${m.progress}%</small>
                </td>
                <td>${m.scheduled_cutover ? new Date(m.scheduled_cutover).toLocaleString() : '-'}</td>
                <td class="action-buttons">
                    ${m.status === 'ready' ? `
                        <button class="btn btn-small btn-secondary" onclick="triggerSync(${m.id})">Sync</button>
                        <button class="btn btn-small btn-primary" onclick="triggerCutover(${m.id})">Cutover</button>
                    ` : ''}
                    ${['pending', 'syncing', 'ready'].includes(m.status) ? `
                        <button class="btn btn-small btn-danger" onclick="cancelMigration(${m.id})">Cancel</button>
                    ` : ''}
                </td>
            </tr>
        `).join('') || '<tr><td colspan="7">No migrations</td></tr>';
    } catch (error) {
        console.error('Failed to load migrations:', error);
    }
}

async function showCreateMigrationModal() {
    // Load VMs, sources, and targets for selects
    const [vms, sources, targets] = await Promise.all([
        api.getVMs(),
        api.getSources(),
        api.getTargets(),
    ]);

    document.getElementById('migration-vm').innerHTML = (vms || [])
        .map(vm => `<option value="${vm.id}">${vm.name}</option>`).join('');
    document.getElementById('migration-source').innerHTML = (sources || [])
        .map(s => `<option value="${s.id}">${s.name}</option>`).join('');
    document.getElementById('migration-target').innerHTML = (targets || [])
        .map(t => `<option value="${t.id}">${t.name} (${t.type})</option>`).join('');

    openModal('create-migration-modal');
}

async function createMigration(event) {
    event.preventDefault();
    try {
        const cutover = document.getElementById('migration-cutover').value;
        await api.createMigration({
            vm_id: parseInt(document.getElementById('migration-vm').value),
            source_env_id: parseInt(document.getElementById('migration-source').value),
            target_env_id: parseInt(document.getElementById('migration-target').value),
            preserve_mac: document.getElementById('migration-preserve-mac').checked,
            preserve_port_groups: document.getElementById('migration-preserve-portgroups').checked,
            sync_interval_minutes: parseInt(document.getElementById('migration-sync-interval').value),
            scheduled_cutover: cutover ? new Date(cutover).toISOString() : null,
        });
        closeModal();
        loadMigrations();
    } catch (error) {
        alert('Failed to create migration: ' + error.message);
    }
}

async function triggerSync(id) {
    try {
        await api.triggerSync(id);
        alert('Sync started');
        loadMigrations();
    } catch (error) {
        alert('Failed to trigger sync: ' + error.message);
    }
}

async function triggerCutover(id) {
    if (!confirm('Are you sure you want to perform the cutover? This will power off the source VM.')) return;
    try {
        await api.triggerCutover(id);
        alert('Cutover started');
        loadMigrations();
    } catch (error) {
        alert('Failed to trigger cutover: ' + error.message);
    }
}

async function cancelMigration(id) {
    if (!confirm('Are you sure you want to cancel this migration?')) return;
    try {
        await api.cancelMigration(id);
        loadMigrations();
    } catch (error) {
        alert('Failed to cancel: ' + error.message);
    }
}

// Schedules
async function loadSchedules() {
    try {
        const tasks = await api.getScheduledTasks();
        const tbody = document.querySelector('#schedules-table tbody');
        tbody.innerHTML = (tasks || []).map(t => `
            <tr>
                <td>${t.task_type}</td>
                <td>${t.job_id}</td>
                <td>${new Date(t.scheduled_time).toLocaleString()}</td>
                <td><span class="status-badge status-${t.status}">${t.status}</span></td>
                <td>${t.created_by}</td>
                <td class="action-buttons">
                    ${t.status === 'pending' ? `
                        <button class="btn btn-small btn-danger" onclick="cancelScheduledTask(${t.id})">Cancel</button>
                    ` : ''}
                </td>
            </tr>
        `).join('') || '<tr><td colspan="6">No scheduled tasks</td></tr>';
    } catch (error) {
        console.error('Failed to load schedules:', error);
    }
}

async function cancelScheduledTask(id) {
    if (!confirm('Are you sure you want to cancel this scheduled task?')) return;
    try {
        await api.cancelScheduledTask(id);
        loadSchedules();
    } catch (error) {
        alert('Failed to cancel: ' + error.message);
    }
}

// Admin
function showAdminTab(tab) {
    document.querySelectorAll('.admin-tab-content').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));

    document.getElementById(`${tab}-tab`).classList.add('active');
    event.target.classList.add('active');

    switch (tab) {
        case 'env-vars':
            loadEnvVariables();
            break;
        case 'users':
            loadUsers();
            break;
        case 'logs':
            loadActivityLogs();
            break;
    }
}

async function loadAdminData() {
    loadEnvVariables();
}

async function loadEnvVariables() {
    try {
        const vars = await api.getEnvVariables();
        const tbody = document.querySelector('#env-vars-table tbody');
        tbody.innerHTML = (vars || []).map(v => `
            <tr>
                <td>${v.name}</td>
                <td>${v.value}</td>
                <td>${v.description || '-'}</td>
                <td>${v.is_secret ? 'Yes' : 'No'}</td>
                <td class="action-buttons">
                    <button class="btn btn-small btn-danger" onclick="deleteEnvVariable(${v.id})">Delete</button>
                </td>
            </tr>
        `).join('') || '<tr><td colspan="5">No environment variables</td></tr>';
    } catch (error) {
        console.error('Failed to load env variables:', error);
    }
}

function showAddEnvVarModal() {
    document.getElementById('env-var-form').reset();
    openModal('env-var-modal');
}

async function saveEnvVar(event) {
    event.preventDefault();
    try {
        await api.createEnvVariable({
            name: document.getElementById('env-var-name').value,
            value: document.getElementById('env-var-value').value,
            description: document.getElementById('env-var-description').value,
            is_secret: document.getElementById('env-var-secret').checked,
        });
        closeModal();
        loadEnvVariables();
    } catch (error) {
        alert('Failed to save: ' + error.message);
    }
}

async function deleteEnvVariable(id) {
    if (!confirm('Are you sure you want to delete this environment variable?')) return;
    try {
        await api.deleteEnvVariable(id);
        loadEnvVariables();
    } catch (error) {
        alert('Failed to delete: ' + error.message);
    }
}

async function loadUsers() {
    try {
        const users = await api.getUsers();
        const tbody = document.querySelector('#users-table tbody');
        tbody.innerHTML = (users || []).map(u => `
            <tr>
                <td>${u.username}</td>
                <td>${u.display_name || '-'}</td>
                <td>${u.email || '-'}</td>
                <td>${u.is_admin ? 'Yes' : 'No'}</td>
                <td>${u.last_login ? new Date(u.last_login).toLocaleString() : 'Never'}</td>
                <td class="action-buttons">
                    <button class="btn btn-small btn-secondary" onclick="toggleAdmin(${u.id}, ${!u.is_admin})">
                        ${u.is_admin ? 'Remove Admin' : 'Make Admin'}
                    </button>
                </td>
            </tr>
        `).join('') || '<tr><td colspan="6">No users</td></tr>';
    } catch (error) {
        console.error('Failed to load users:', error);
    }
}

async function toggleAdmin(id, isAdmin) {
    try {
        await api.toggleUserAdmin(id, isAdmin);
        loadUsers();
    } catch (error) {
        alert('Failed to update user: ' + error.message);
    }
}

async function loadActivityLogs() {
    try {
        const logs = await api.getActivityLogs();
        const tbody = document.querySelector('#logs-table tbody');
        tbody.innerHTML = (logs || []).map(l => `
            <tr>
                <td>${new Date(l.created_at).toLocaleString()}</td>
                <td>${l.username}</td>
                <td>${l.action}</td>
                <td>${l.details || '-'}</td>
            </tr>
        `).join('') || '<tr><td colspan="4">No activity logs</td></tr>';
    } catch (error) {
        console.error('Failed to load logs:', error);
    }
}

// Modal helpers
function openModal(modalId) {
    document.getElementById('modal-overlay').classList.add('active');
    document.getElementById(modalId).classList.add('active');
}

function closeModal() {
    document.getElementById('modal-overlay').classList.remove('active');
    document.querySelectorAll('.modal').forEach(m => m.classList.remove('active'));
}
