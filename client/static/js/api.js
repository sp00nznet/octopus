// API Client for Octopus

const API_BASE = '/api/v1';

class OctopusAPI {
    constructor() {
        this.token = localStorage.getItem('octopus_token');
    }

    setToken(token) {
        this.token = token;
        localStorage.setItem('octopus_token', token);
    }

    clearToken() {
        this.token = null;
        localStorage.removeItem('octopus_token');
        localStorage.removeItem('octopus_user');
    }

    async request(endpoint, options = {}) {
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers,
        };

        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }

        try {
            const response = await fetch(`${API_BASE}${endpoint}`, {
                ...options,
                headers,
            });

            if (response.status === 401) {
                this.clearToken();
                window.location.href = '/login';
                return null;
            }

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Request failed');
            }

            return data;
        } catch (error) {
            console.error('API Error:', error);
            throw error;
        }
    }

    // Authentication
    async login(username, password) {
        const data = await this.request('/auth/login', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
        });
        if (data && data.token) {
            this.setToken(data.token);
        }
        return data;
    }

    // Source Environments
    async getSources() {
        return this.request('/sources');
    }

    async getSource(id) {
        return this.request(`/sources/${id}`);
    }

    async createSource(data) {
        return this.request('/sources', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async updateSource(id, data) {
        return this.request(`/sources/${id}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    }

    async deleteSource(id) {
        return this.request(`/sources/${id}`, {
            method: 'DELETE',
        });
    }

    async syncSource(id) {
        return this.request(`/sources/${id}/sync`, {
            method: 'POST',
        });
    }

    // Target Environments
    async getTargets() {
        return this.request('/targets');
    }

    async getTarget(id) {
        return this.request(`/targets/${id}`);
    }

    async createTarget(data) {
        return this.request('/targets', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async updateTarget(id, data) {
        return this.request(`/targets/${id}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    }

    async deleteTarget(id) {
        return this.request(`/targets/${id}`, {
            method: 'DELETE',
        });
    }

    // Unified Environments (can be source or target)
    async getEnvironments() {
        return this.request('/environments');
    }

    async getEnvironment(id) {
        return this.request(`/environments/${id}`);
    }

    async createEnvironment(data) {
        return this.request('/environments', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async updateEnvironment(id, data) {
        return this.request(`/environments/${id}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    }

    async deleteEnvironment(id) {
        return this.request(`/environments/${id}`, {
            method: 'DELETE',
        });
    }

    async syncEnvironment(id) {
        return this.request(`/environments/${id}/sync`, {
            method: 'POST',
        });
    }

    // Virtual Machines
    async getVMs(sourceId = null) {
        const query = sourceId ? `?source_id=${sourceId}` : '';
        return this.request(`/vms${query}`);
    }

    async getVM(id) {
        return this.request(`/vms/${id}`);
    }

    async estimateVMSize(id, targetType, isVXRail = false) {
        return this.request(`/vms/${id}/estimate`, {
            method: 'POST',
            body: JSON.stringify({ target_type: targetType, is_vxrail: isVXRail }),
        });
    }

    // Migrations
    async getMigrations(status = null) {
        const query = status ? `?status=${status}` : '';
        return this.request(`/migrations${query}`);
    }

    async getMigration(id) {
        return this.request(`/migrations/${id}`);
    }

    async createMigration(data) {
        return this.request('/migrations', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async updateMigration(id, data) {
        return this.request(`/migrations/${id}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    }

    async cancelMigration(id) {
        return this.request(`/migrations/${id}/cancel`, {
            method: 'POST',
        });
    }

    async triggerSync(id) {
        return this.request(`/migrations/${id}/sync`, {
            method: 'POST',
        });
    }

    async triggerCutover(id) {
        return this.request(`/migrations/${id}/cutover`, {
            method: 'POST',
        });
    }

    // Scheduled Tasks
    async getScheduledTasks() {
        return this.request('/schedules');
    }

    async getScheduledTask(id) {
        return this.request(`/schedules/${id}`);
    }

    async createScheduledTask(data) {
        return this.request('/schedules', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async cancelScheduledTask(id) {
        return this.request(`/schedules/${id}/cancel`, {
            method: 'POST',
        });
    }

    // Admin - Environment Variables
    async getEnvVariables() {
        return this.request('/admin/env');
    }

    async createEnvVariable(data) {
        return this.request('/admin/env', {
            method: 'POST',
            body: JSON.stringify(data),
        });
    }

    async updateEnvVariable(id, data) {
        return this.request(`/admin/env/${id}`, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    }

    async deleteEnvVariable(id) {
        return this.request(`/admin/env/${id}`, {
            method: 'DELETE',
        });
    }

    // Admin - Users
    async getUsers() {
        return this.request('/admin/users');
    }

    async getUser(id) {
        return this.request(`/admin/users/${id}`);
    }

    async toggleUserAdmin(id, isAdmin) {
        return this.request(`/admin/users/${id}/admin`, {
            method: 'PUT',
            body: JSON.stringify({ is_admin: isAdmin }),
        });
    }

    // Admin - Activity Logs
    async getActivityLogs(limit = 100) {
        return this.request(`/admin/logs?limit=${limit}`);
    }

    // Health Check
    async healthCheck() {
        return this.request('/health');
    }
}

// Global API instance
const api = new OctopusAPI();
