import axios from 'axios';

const API_BASE_URL = process.env.REACT_APP_API_URL || '/api';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor to add auth token if needed
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('git-token');
    if (token) {
      config.headers['X-Git-Token'] = token;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Response interceptor for error handling
api.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error('API Error:', error);
    return Promise.reject(error);
  }
);

export const appsAPI = {
  // Apps
  createApp: (data) => api.post('/apps', data),
  getApps: () => api.get('/apps'),
  getApp: (id) => api.get(`/apps/${id}`),
  getAppBranches: (id) => api.get(`/apps/${id}/branches`),
  validateDockerfile: (id, branch) => api.post(`/apps/${id}/validate?branch=${branch || ''}`),
  
  // Releases
  createRelease: (appId, data) => api.post(`/apps/${appId}/releases`, data),
  getReleases: (appId, page = 1, limit = 10) => api.get(`/apps/${appId}/releases?page=${page}&limit=${limit}`),
  getRelease: (id) => api.get(`/releases/${id}`),
  deployRelease: (id) => api.post(`/releases/${id}/deploy`),
  rollbackRelease: (id) => api.post(`/releases/${id}/rollback`),
  
  // Build Monitoring
  getBuildStatus: (id) => api.get(`/releases/${id}/build-status`),
  getBuildStages: (id) => api.get(`/releases/${id}/build-stages`),
  getBuildLogsForStage: (id, stage) => api.get(`/releases/${id}/build-logs/${stage}`),
  getBuildLogs: (id, params = {}) => {
    const query = new URLSearchParams(params).toString();
    return api.get(`/releases/${id}/build-logs${query ? `?${query}` : ''}`);
  },
};

export const createWebSocketConnection = (releaseId) => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = process.env.REACT_APP_WS_URL || window.location.host;
  return new WebSocket(`${protocol}//${host}/api/releases/${releaseId}/stream`);
};

export const createEventSourceConnection = (releaseId) => {
  const baseUrl = process.env.REACT_APP_API_URL || '';
  return new EventSource(`${baseUrl}/api/releases/${releaseId}/stream-sse`);
};

export default api;