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
        case 'environments':
            loadEnvironments();
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

// Environment Tabs
function showEnvTab(tab) {
    document.querySelectorAll('.env-tab-content').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('#environments-page .tab-btn').forEach(b => b.classList.remove('active'));

    document.getElementById(`${tab}-tab`).classList.add('active');
    event.target.classList.add('active');

    if (tab === 'sources') {
        loadSources();
    } else {
        loadTargets();
    }
}

function loadEnvironments() {
    loadSources();
    loadTargets();
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
                <td>${m.vm_name || 'N/A'}</td>
                <td>${m.source_name || 'N/A'}</td>
                <td>${m.target_name || 'N/A'}</td>
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
            .filter(t => ['pending', 'running'].includes(t.status))
            .slice(0, 5);
        const tasksBody = document.querySelector('#upcoming-tasks-table tbody');
        tasksBody.innerHTML = upcomingTasks.map(t => `
            <tr>
                <td>${t.task_type}</td>
                <td>${t.target_name || t.job_id || '-'}</td>
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
                <td>${s.type === 'vmware-vxrail' ? 'VMware vXRAIL' : 'VMware vCenter'}</td>
                <td>${s.host}</td>
                <td>${s.datacenter || '-'}</td>
                <td>${s.type === 'vmware-vxrail' ? '<span class="status-badge status-syncing">Yes</span>' : '-'}</td>
                <td class="action-buttons">
                    <button class="btn btn-small btn-secondary" onclick="editSource(${s.id})">Edit</button>
                    <button class="btn btn-small btn-secondary" onclick="syncSource(${s.id})">Sync</button>
                    <button class="btn btn-small btn-warning" onclick="convertSourceToTarget(${s.id})" title="Convert to Target">Swap</button>
                    <button class="btn btn-small btn-danger" onclick="deleteSource(${s.id})">Delete</button>
                </td>
            </tr>
        `).join('') || '<tr><td colspan="6">No source environments</td></tr>';

        // Update VM filter
        const filter = document.getElementById('vm-source-filter');
        filter.innerHTML = '<option value="">All Sources</option>' +
            (sources || []).map(s => `<option value="${s.id}">${s.name}</option>`).join('');

    } catch (error) {
        console.error('Failed to load sources:', error);
    }
}

function showAddSourceModal() {
    document.getElementById('source-form').reset();
    document.getElementById('source-id').value = '';
    document.getElementById('source-modal-title').textContent = 'Add Source Environment';
    document.getElementById('source-submit-btn').textContent = 'Add Source';
    document.getElementById('source-password').required = true;
    document.getElementById('source-password').placeholder = '';
    openModal('source-modal');
}

async function editSource(id) {
    try {
        const source = await api.getSource(id);
        document.getElementById('source-id').value = source.id;
        document.getElementById('source-name').value = source.name;
        document.getElementById('source-type').value = source.type;
        document.getElementById('source-host').value = source.host;
        document.getElementById('source-username').value = source.username || '';
        document.getElementById('source-password').value = '';
        document.getElementById('source-password').required = false;
        document.getElementById('source-password').placeholder = 'Leave blank to keep existing';
        document.getElementById('source-datacenter').value = source.datacenter || '';
        document.getElementById('source-cluster').value = source.cluster || '';
        document.getElementById('source-modal-title').textContent = 'Edit Source Environment';
        document.getElementById('source-submit-btn').textContent = 'Save Changes';
        openModal('source-modal');
    } catch (error) {
        alert('Failed to load source: ' + error.message);
    }
}

async function saveSource(event) {
    event.preventDefault();
    const id = document.getElementById('source-id').value;
    const data = {
        name: document.getElementById('source-name').value,
        type: document.getElementById('source-type').value,
        host: document.getElementById('source-host').value,
        username: document.getElementById('source-username').value,
        datacenter: document.getElementById('source-datacenter').value,
        cluster: document.getElementById('source-cluster').value,
    };

    const password = document.getElementById('source-password').value;
    if (password) {
        data.password = password;
    }

    try {
        if (id) {
            await api.updateSource(id, data);
        } else {
            data.password = password;
            await api.createSource(data);
        }
        closeModal();
        loadSources();
    } catch (error) {
        alert('Failed to save source: ' + error.message);
    }
}

async function syncSource(id) {
    try {
        const result = await api.syncSource(id);
        alert(`Sync started. ${result.vm_count || 0} VMs found.`);
        loadVMs();
        loadSchedules();
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
    loadSchedules();
}

// Targets
async function loadTargets() {
    try {
        const targets = await api.getTargets();
        const tbody = document.querySelector('#targets-table tbody');
        tbody.innerHTML = (targets || []).map(t => {
            const config = t.config ? (typeof t.config === 'string' ? JSON.parse(t.config) : t.config) : {};
            const hostOrRegion = config.host || config.region || config.zone || config.resource_group || '-';
            return `
                <tr>
                    <td>${t.name}</td>
                    <td>${t.type}</td>
                    <td>${hostOrRegion}</td>
                    <td>${new Date(t.created_at).toLocaleDateString()}</td>
                    <td class="action-buttons">
                        <button class="btn btn-small btn-secondary" onclick="editTarget(${t.id})">Edit</button>
                        ${t.type === 'vmware' ? `<button class="btn btn-small btn-warning" onclick="convertTargetToSource(${t.id})" title="Convert to Source">Swap</button>` : ''}
                        <button class="btn btn-small btn-danger" onclick="deleteTarget(${t.id})">Delete</button>
                    </td>
                </tr>
            `;
        }).join('') || '<tr><td colspan="5">No target environments</td></tr>';
    } catch (error) {
        console.error('Failed to load targets:', error);
    }
}

function showAddTargetModal() {
    document.getElementById('target-form').reset();
    document.getElementById('target-id').value = '';
    document.getElementById('target-modal-title').textContent = 'Add Target Environment';
    document.getElementById('target-submit-btn').textContent = 'Add Target';
    updateTargetConfigFields();
    openModal('target-modal');
}

async function editTarget(id) {
    try {
        const target = await api.getTarget(id);
        document.getElementById('target-id').value = target.id;
        document.getElementById('target-name').value = target.name;
        document.getElementById('target-type').value = target.type;
        updateTargetConfigFields();

        // Populate config fields if we have config data
        if (target.config) {
            const config = typeof target.config === 'string' ? JSON.parse(target.config) : target.config;
            Object.keys(config).forEach(key => {
                const input = document.querySelector(`#target-config-fields [name="${key}"]`);
                if (input) {
                    input.value = config[key];
                }
            });
        }

        document.getElementById('target-modal-title').textContent = 'Edit Target Environment';
        document.getElementById('target-submit-btn').textContent = 'Save Changes';
        openModal('target-modal');
    } catch (error) {
        alert('Failed to load target: ' + error.message);
    }
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
                <input type="password" name="password" placeholder="Leave blank to keep existing">
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
                <input type="password" name="secret_access_key" placeholder="Leave blank to keep existing">
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
                <textarea name="credentials" placeholder="Leave blank to keep existing"></textarea>
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
                <input type="password" name="client_secret" placeholder="Leave blank to keep existing">
            </div>
        `,
    };

    container.innerHTML = fields[type] || '';
}

async function saveTarget(event) {
    event.preventDefault();
    const id = document.getElementById('target-id').value;
    const type = document.getElementById('target-type').value;

    // Collect config fields
    const config = {};
    const inputs = document.querySelectorAll('#target-config-fields input, #target-config-fields textarea');
    inputs.forEach(input => {
        if (input.value) {
            config[input.name] = input.value;
        }
    });

    try {
        const data = {
            name: document.getElementById('target-name').value,
            type: type,
            config: config,
        };

        if (id) {
            await api.updateTarget(id, data);
        } else {
            await api.createTarget(data);
        }
        closeModal();
        loadTargets();
    } catch (error) {
        alert('Failed to save target: ' + error.message);
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
    content.innerHTML = '<p style="padding: 1.5rem;">Loading...</p>';
    openModal('size-estimation-modal');

    try {
        // Get VM info to check if from vXRAIL
        const vm = await api.getVM(vmId);
        const sources = await api.getSources();
        const source = sources.find(s => s.id === vm.source_env_id);
        const isVXRail = source?.type === 'vmware-vxrail';

        const results = await Promise.all([
            api.estimateVMSize(vmId, 'vmware', isVXRail),
            api.estimateVMSize(vmId, 'aws', isVXRail),
            api.estimateVMSize(vmId, 'gcp', isVXRail),
            api.estimateVMSize(vmId, 'azure', isVXRail),
        ]);

        content.innerHTML = `
            <div class="size-estimation-result">
                <h4>${vm.name} - Size Estimations</h4>
                ${isVXRail ? '<p style="color: var(--warning-color); margin-bottom: 1rem;"><strong>vXRAIL Source:</strong> Sizes adjusted for RAID-1 overhead (typically 50% reduction)</p>' : ''}
                ${results.map((r, i) => {
                    const targets = ['VMware (Standard)', 'AWS', 'GCP', 'Azure'];
                    const savings = r.source_size_gb > 0 ? ((r.source_size_gb - r.estimated_size_gb) / r.source_size_gb * 100).toFixed(1) : 0;
                    return `
                        <div class="target-estimation" style="margin-bottom: 1rem; padding: 1rem; background: var(--background); border-radius: 0.375rem;">
                            <h5 style="margin-bottom: 0.5rem;">${targets[i]}</h5>
                            <div class="stat">
                                <span class="stat-label">Source Size (Reported)</span>
                                <span class="stat-value">${r.source_size_gb.toFixed(2)} GB</span>
                            </div>
                            <div class="stat">
                                <span class="stat-label">Estimated Actual Size</span>
                                <span class="stat-value">${r.estimated_size_gb.toFixed(2)} GB</span>
                            </div>
                            <div class="stat">
                                <span class="stat-label">Estimated Savings</span>
                                <span class="stat-value" style="color: var(--success-color);">${savings}%</span>
                            </div>
                            ${r.notes ? `<div class="notes" style="margin-top: 0.5rem; font-size: 0.875rem; color: var(--text-secondary);">${r.notes}</div>` : ''}
                        </div>
                    `;
                }).join('')}
            </div>
        `;
    } catch (error) {
        content.innerHTML = `<p class="error" style="padding: 1.5rem;">Failed to estimate size: ${error.message}</p>`;
    }
}

// Batch vXRAIL Estimation
async function showBatchEstimationModal() {
    const [sources, targets] = await Promise.all([
        api.getSources(),
        api.getTargets(),
    ]);

    const vxrailSources = (sources || []).filter(s => s.type === 'vmware-vxrail');

    const sourceFilter = document.getElementById('batch-source-filter');
    sourceFilter.innerHTML = '<option value="">Select vXRAIL Source...</option>' +
        vxrailSources.map(s => `<option value="${s.id}">${s.name}</option>`).join('');

    if (vxrailSources.length === 0) {
        sourceFilter.innerHTML = '<option value="">No vXRAIL sources configured</option>';
    }

    const targetFilter = document.getElementById('batch-target-filter');
    targetFilter.innerHTML = '<option value="">Select Target...</option>' +
        (targets || []).map(t => `<option value="${t.id}" data-type="${t.type}" data-name="${t.name}">${t.name} (${t.type})</option>`).join('');

    if ((targets || []).length === 0) {
        targetFilter.innerHTML = '<option value="">No targets configured</option>';
    }

    document.getElementById('batch-vm-list').innerHTML = '<p style="color: var(--text-secondary);">Select a source environment to load VMs</p>';
    document.getElementById('batch-results').style.display = 'none';

    openModal('batch-estimation-modal');
}

async function loadBatchVMs() {
    const sourceId = document.getElementById('batch-source-filter').value;
    const container = document.getElementById('batch-vm-list');

    if (!sourceId) {
        container.innerHTML = '<p style="color: var(--text-secondary);">Select a source environment to load VMs</p>';
        return;
    }

    try {
        const vms = await api.getVMs(sourceId);
        container.innerHTML = (vms || []).map(vm => `
            <label style="display: block; padding: 0.5rem; border-bottom: 1px solid var(--border-color);">
                <input type="checkbox" class="batch-vm-checkbox" value="${vm.id}" data-name="${vm.name}" data-size="${vm.disk_size_gb}">
                ${vm.name} (${vm.disk_size_gb.toFixed(1)} GB)
            </label>
        `).join('') || '<p>No VMs found in this source</p>';
    } catch (error) {
        container.innerHTML = `<p class="error">Failed to load VMs: ${error.message}</p>`;
    }
}

function selectAllBatchVMs() {
    const checkboxes = document.querySelectorAll('.batch-vm-checkbox');
    const allChecked = Array.from(checkboxes).every(cb => cb.checked);
    checkboxes.forEach(cb => cb.checked = !allChecked);
}

async function runBatchEstimation() {
    const checkboxes = document.querySelectorAll('.batch-vm-checkbox:checked');
    if (checkboxes.length === 0) {
        alert('Please select at least one VM');
        return;
    }

    const targetSelect = document.getElementById('batch-target-filter');
    const selectedTarget = targetSelect.options[targetSelect.selectedIndex];
    if (!targetSelect.value) {
        alert('Please select a target environment');
        return;
    }

    const targetType = selectedTarget.dataset.type;
    const targetName = selectedTarget.dataset.name;

    const resultsDiv = document.getElementById('batch-results');
    const tbody = document.querySelector('#batch-results-table tbody');
    tbody.innerHTML = '<tr><td colspan="4">Calculating...</td></tr>';
    resultsDiv.style.display = 'block';
    document.getElementById('batch-target-name').textContent = `→ ${targetName} (${targetType})`;

    const results = [];
    let totalSource = 0;
    let totalEstimated = 0;

    for (const cb of checkboxes) {
        try {
            const estimation = await api.estimateVMSize(cb.value, targetType, true);
            results.push({
                name: cb.dataset.name,
                source: estimation.source_size_gb,
                estimated: estimation.estimated_size_gb,
            });
            totalSource += estimation.source_size_gb;
            totalEstimated += estimation.estimated_size_gb;
        } catch (error) {
            results.push({
                name: cb.dataset.name,
                source: parseFloat(cb.dataset.size),
                estimated: parseFloat(cb.dataset.size) * 0.5,
                error: true,
            });
        }
    }

    tbody.innerHTML = results.map(r => {
        const savings = r.source > 0 ? ((r.source - r.estimated) / r.source * 100).toFixed(1) : 0;
        return `
            <tr${r.error ? ' style="color: var(--warning-color);"' : ''}>
                <td>${r.name}${r.error ? ' (estimated)' : ''}</td>
                <td>${r.source.toFixed(2)}</td>
                <td>${r.estimated.toFixed(2)}</td>
                <td>${savings}%</td>
            </tr>
        `;
    }).join('');

    const totalSavings = totalSource > 0 ? ((totalSource - totalEstimated) / totalSource * 100).toFixed(1) : 0;
    document.getElementById('batch-total-source').textContent = totalSource.toFixed(2) + ' GB';
    document.getElementById('batch-total-estimated').textContent = totalEstimated.toFixed(2) + ' GB';
    document.getElementById('batch-total-savings').textContent = totalSavings + '%';
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
                <td>${m.vm_name || 'N/A'}</td>
                <td>${m.source_name || 'N/A'}</td>
                <td>${m.target_name || 'N/A'}</td>
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
                <td><span class="status-badge status-${t.task_type === 'scan' ? 'syncing' : 'ready'}">${t.task_type}</span></td>
                <td>${t.target_name || t.source_name || (t.job_id ? `Job #${t.job_id}` : '-')}</td>
                <td>${new Date(t.scheduled_time).toLocaleString()}</td>
                <td><span class="status-badge status-${t.status}">${t.status}</span></td>
                <td>
                    ${t.status === 'running' ? `
                        <div class="progress-bar" style="width: 100px; display: inline-block;">
                            <div class="progress-bar-fill" style="width: ${t.progress || 0}%"></div>
                        </div>
                        <small>${t.progress || 0}%</small>
                    ` : '-'}
                </td>
                <td>${t.created_by || '-'}</td>
                <td class="action-buttons">
                    ${t.status === 'pending' ? `
                        <button class="btn btn-small btn-danger" onclick="cancelScheduledTask(${t.id})">Cancel</button>
                    ` : ''}
                </td>
            </tr>
        `).join('') || '<tr><td colspan="7">No scheduled tasks</td></tr>';
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
                <td>${v.is_secret ? '********' : v.value}</td>
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
                <td>${l.username || '-'}</td>
                <td>${l.action}</td>
                <td>${l.details || '-'}</td>
            </tr>
        `).join('') || '<tr><td colspan="4">No activity logs</td></tr>';
    } catch (error) {
        console.error('Failed to load logs:', error);
    }
}

// Environment Swap Functions
async function convertSourceToTarget(sourceId) {
    if (!confirm('Convert this source environment to a target? This will copy the configuration to targets.')) return;

    try {
        const source = await api.getSource(sourceId);
        // Create target with same VMware config
        await api.createTarget({
            name: source.name + ' (Target)',
            type: 'vmware',
            config: {
                host: source.host,
                username: source.username,
                datacenter: source.datacenter,
            },
        });
        alert('Source environment copied to targets. You can now use it as a migration target.');
        loadEnvironments();
        // Switch to targets tab
        showEnvTab('targets');
    } catch (error) {
        alert('Failed to convert: ' + error.message);
    }
}

async function convertTargetToSource(targetId) {
    if (!confirm('Convert this target environment to a source? This will copy the configuration to sources.')) return;

    try {
        const target = await api.getTarget(targetId);
        const config = target.config ? (typeof target.config === 'string' ? JSON.parse(target.config) : target.config) : {};

        // Create source with VMware config
        await api.createSource({
            name: target.name + ' (Source)',
            type: 'vmware',
            host: config.host || '',
            username: config.username || '',
            password: '', // User will need to set password
            datacenter: config.datacenter || '',
        });
        alert('Target environment copied to sources. Please edit to set the password.');
        loadEnvironments();
        // Switch to sources tab
        showEnvTab('sources');
    } catch (error) {
        alert('Failed to convert: ' + error.message);
    }
}

// CSV Export Functions
function downloadCSV(filename, csvContent) {
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = filename;
    link.style.display = 'none';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
}

function exportBatchEstimationCSV() {
    const table = document.getElementById('batch-results-table');
    const rows = table.querySelectorAll('tbody tr');
    const targetName = document.getElementById('batch-target-name').textContent;

    if (rows.length === 0) {
        alert('No results to export');
        return;
    }

    let csv = 'VM Name,vXRAIL Size (GB),Estimated on Target (GB),Savings (%)\n';

    rows.forEach(row => {
        const cells = row.querySelectorAll('td');
        if (cells.length >= 4) {
            const name = cells[0].textContent.replace(/,/g, '');
            const sourceSize = cells[1].textContent;
            const estSize = cells[2].textContent;
            const savings = cells[3].textContent;
            csv += `"${name}",${sourceSize},${estSize},${savings}\n`;
        }
    });

    // Add totals
    const totalSource = document.getElementById('batch-total-source').textContent;
    const totalEst = document.getElementById('batch-total-estimated').textContent;
    const totalSavings = document.getElementById('batch-total-savings').textContent;
    csv += `"TOTAL",${totalSource.replace(' GB', '')},${totalEst.replace(' GB', '')},${totalSavings}\n`;

    const date = new Date().toISOString().split('T')[0];
    downloadCSV(`vxrail-estimation-${date}.csv`, csv);
}

async function exportSchedulesCSV() {
    try {
        const tasks = await api.getScheduledTasks();

        if (!tasks || tasks.length === 0) {
            alert('No scheduled tasks to export');
            return;
        }

        let csv = 'ID,Type,Target,Scheduled Time,Status,Progress,Created By\n';

        tasks.forEach(t => {
            const target = t.target_name || t.source_name || (t.job_id ? `Job #${t.job_id}` : '-');
            const scheduledTime = new Date(t.scheduled_time).toISOString();
            csv += `${t.id},"${t.task_type}","${target}","${scheduledTime}","${t.status}",${t.progress || 0},"${t.created_by || '-'}"\n`;
        });

        const date = new Date().toISOString().split('T')[0];
        downloadCSV(`scheduled-tasks-${date}.csv`, csv);
    } catch (error) {
        alert('Failed to export: ' + error.message);
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
