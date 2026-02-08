import React, { useState, useEffect } from 'react';
import { 
  Card, 
  Button, 
  Table, 
  message, 
  Space, 
  Tag, 
  Select, 
  Alert,
  Modal,
  Descriptions,
  Badge,
  Form,
  Input,
  InputNumber,
  Tabs
} from 'antd';
import { 
  ArrowLeftOutlined, 
  ReloadOutlined, 
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  BuildOutlined,
  SettingOutlined,
  PlusOutlined,
  DeleteOutlined,
  DownloadOutlined,
  FileTextOutlined,
  ConsoleSqlOutlined
} from '@ant-design/icons';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';
import { getStatusBadge } from '../utils/statusHelpers';

const { Option } = Select;

const AppDetails = ({ onAppDeleted }) => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [app, setApp] = useState(null);
  const [releases, setReleases] = useState([]);
  const [deployments, setDeployments] = useState([]);
  const [branches, setBranches] = useState([]);
  const [selectedBranch, setSelectedBranch] = useState(null); // Now stores branch object {name, sha}
  const [dockerfileValidation, setDockerfileValidation] = useState(null);
  const [loading, setLoading] = useState(false);
  const [releasesLoading, setReleasesLoading] = useState(false);
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [validating, setValidating] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [deployModalVisible, setDeployModalVisible] = useState(false);
  const [selectedReleaseId, setSelectedReleaseId] = useState(null);
  const [deployForm] = Form.useForm();
  const [scaleModalVisible, setScaleModalVisible] = useState(false);
  const [selectedDeployment, setSelectedDeployment] = useState(null);
  const [scaleForm] = Form.useForm();
  const [deleteModalVisible, setDeleteModalVisible] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [scaling, setScaling] = useState(false);
  const [deleteDeploymentModalVisible, setDeleteDeploymentModalVisible] = useState(false);
  const [deletingDeployment, setDeletingDeployment] = useState(false);
  const [podLogsModalVisible, setPodLogsModalVisible] = useState(false);
  const [selectedPod, setSelectedPod] = useState(null);
  const [podLogs, setPodLogs] = useState('');
  const [loadingLogs, setLoadingLogs] = useState(false);
  const [ttyWs, setTtyWs] = useState(null);
  const [ttyOutput, setTtyOutput] = useState('');
  const [ttyConnected, setTtyConnected] = useState(false);
  const [activeTab, setActiveTab] = useState('logs');
  const [podDescribe, setPodDescribe] = useState(null);
  const [loadingDescribe, setLoadingDescribe] = useState(false);
  const [ingressInfo, setIngressInfo] = useState(null);
  const [loadingIngress, setLoadingIngress] = useState(false);
  const [reloadModalVisible, setReloadModalVisible] = useState(false);
  const [reloading, setReloading] = useState(false);
  const [reloadForm] = Form.useForm();
  const [selectedReloadDeployment, setSelectedReloadDeployment] = useState(null);

  // Helper function to clean ANSI escape sequences
  const cleanAnsiEscapes = (text) => {
    // Remove ANSI escape sequences like [6n, [?25h, etc.
    return text.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '').replace(/\x1b\[[0-9]*n/g, '');
  };

  useEffect(() => {
    if (id) {
      fetchApp();
      fetchReleases();
      fetchDeployments();
      fetchIngress();
    }
  }, [id]);

  const fetchApp = async () => {
    setLoading(true);
    try {
      const response = await appsAPI.getApp(id);
      setApp(response.data);
      // Will set the actual branch object when branches are fetched
    } catch (error) {
      message.error('Failed to fetch application');
    } finally {
      setLoading(false);
    }
  };

  const fetchReleases = async () => {
    setReleasesLoading(true);
    try {
      const response = await appsAPI.getReleases(id);
      // Filter to show only build-related releases (pending, running, success, failed, deployed)
      const buildReleases = response.data.releases?.filter(release => 
        ['pending', 'running', 'success', 'failed', 'deployed'].includes(release.status)
      ) || [];
      setReleases(buildReleases);
    } catch (error) {
      message.error('Failed to fetch releases');
    } finally {
      setReleasesLoading(false);
    }
  };

  const fetchDeployments = async () => {
    try {
      const response = await appsAPI.getAppDeployments(id);
      setDeployments(response.data.deployments || []);
    } catch (error) {
      message.error('Failed to fetch deployments');
    } finally {
    }
  };

  const fetchIngress = async () => {
    setLoadingIngress(true);
    try {
      const response = await appsAPI.getAppIngress(id);
      setIngressInfo(response.data);
    } catch (error) {
      // Don't show error message as ingress might not exist yet
      setIngressInfo(null);
    } finally {
      setLoadingIngress(false);
    }
  };

  const fetchBranches = async () => {
    setBranchesLoading(true);
    try {
      const response = await appsAPI.getAppBranches(id);
      const fetchedBranches = response.data.branches || [];
      setBranches(fetchedBranches);

      // Set default branch as selected if not already selected
      if (app && !selectedBranch && fetchedBranches.length > 0) {
        const defaultBranch = fetchedBranches.find(b => b.name === app.default_branch);
        if (defaultBranch) {
          setSelectedBranch(defaultBranch);
        } else {
          setSelectedBranch(fetchedBranches[0]);
        }
      }
    } catch (error) {
      message.error('Failed to fetch branches');
    } finally {
      setBranchesLoading(false);
    }
  };

  const validateDockerfile = async (branchObj) => {
    setValidating(true);
    try {
      const branchName = branchObj?.name || branchObj;
      const response = await appsAPI.validateDockerfile(id, branchName);
      setDockerfileValidation(response.data);
      if (response.data.valid) {
        message.success('Dockerfile found and valid');
      } else {
        message.warning('Dockerfile validation failed');
      }
    } catch (error) {
      message.error('Failed to validate Dockerfile');
    } finally {
      setValidating(false);
    }
  };

  const handleDeploy = async () => {
    if (!selectedBranch || !selectedBranch.sha) {
      message.error('Please select a branch first');
      return;
    }

    if (!dockerfileValidation || !dockerfileValidation.valid) {
      Modal.confirm({
        title: 'Dockerfile Not Validated',
        content: 'Dockerfile has not been validated for this branch. Do you want to validate it first?',
        onOk: () => validateDockerfile(selectedBranch),
      });
      return;
    }

    Modal.confirm({
      title: 'Build Application',
      content: `Are you sure you want to build branch "${selectedBranch.name}" (${selectedBranch.sha.substring(0, 7)})?`,
      onOk: async () => {
        setDeploying(true);
        try {
          const response = await appsAPI.createRelease(id, {
            branch: selectedBranch.name,
            commit_sha: selectedBranch.sha,
          });
          message.success('Release created successfully');
          navigate(`/releases/${response.data.id}/build`);
        } catch (error) {
          message.error(error.response?.data?.error || 'Failed to create release');
        } finally {
          setDeploying(false);
        }
      },
    });
  };

  const showDeployModal = (releaseId) => {
    setSelectedReleaseId(releaseId);
    deployForm.setFieldsValue({
      env_vars: app?.env_vars || []
    });
    setDeployModalVisible(true);
  };

  const showScaleModal = (deployment) => {
    setSelectedDeployment(deployment);
    scaleForm.setFieldsValue({
      replicas: deployment.replicas?.desired || 1
    });
    setScaleModalVisible(true);
  };

  const showReloadModal = (deployment) => {
    setSelectedReloadDeployment(deployment);
    reloadForm.setFieldsValue({
      maxUnavailable: 25,
      env_vars: app?.env_vars || []
    });
    setReloadModalVisible(true);
  };

  const handleReloadModalOk = async () => {
    try {
      const values = await reloadForm.validateFields();
      setReloading(true);

      const response = await appsAPI.reloadApp(id, values);
      message.success('Reload initiated');
      setReloadModalVisible(false);
      fetchDeployments();
      fetchReleases();
      navigate(`/releases/${response.data.release.id}/deploy`);
    } catch (error) {
      if (error.errorFields) {
        return;
      }
      message.error(error.response?.data?.error || 'Failed to reload deployment');
    } finally {
      setReloading(false);
    }
  };

  const handleScaleModalOk = async () => {
    try {
      const values = await scaleForm.validateFields();
      
      // If replicas is 0, show delete confirmation dialog
      if (values.replicas === 0) {
        setScaleModalVisible(false);
        setDeleteDeploymentModalVisible(true);
        return;
      }
      
      setScaling(true);
      
      await appsAPI.scaleDeployment(id, values);
      message.success('Scaling initiated');
      setScaleModalVisible(false);
      // Refresh deployments after scaling
      fetchDeployments();
    } catch (error) {
      if (error.errorFields) {
        // Validation failed
        return;
      }
      message.error('Failed to scale deployment');
    } finally {
      setScaling(false);
    }
  };

  const showPodLogsModal = async (pod, deployment) => {
    setSelectedPod({ ...pod, deployment });
    setPodLogsModalVisible(true);
    setActiveTab('logs');
    setLoadingLogs(true);
    
    try {
      const response = await appsAPI.getPodLogs(id, pod.name, deployment?.deployment_name);
      setPodLogs(response.data.logs || 'No logs available');
    } catch (error) {
      setPodLogs(`Error loading logs: ${error.response?.data?.error || error.message}`);
      message.error('Failed to load pod logs');
    } finally {
      setLoadingLogs(false);
    }
  };

  const fetchPodDescribe = async () => {
    if (!selectedPod) return;
    
    setLoadingDescribe(true);
    try {
      const response = await appsAPI.getPodDescribe(id, selectedPod.name);
      setPodDescribe(response.data.describe);
    } catch (error) {
      setPodDescribe({ error: `Error loading pod describe: ${error.response?.data?.error || error.message}` });
      message.error('Failed to load pod describe information');
    } finally {
      setLoadingDescribe(false);
    }
  };

  const connectTTY = () => {
    if (!selectedPod) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // For WebSocket connections, we need to directly connect to the backend port
    const wsHost = `${window.location.hostname}:9091`;
    const containerParam = selectedPod.deployment?.deployment_name ? 
      `?container=${selectedPod.deployment.deployment_name}` : '';
    
    const wsUrl = `${protocol}//${wsHost}/api/apps/${id}/pods/${selectedPod.name}/tty${containerParam}`;
    
    try {
      const ws = new WebSocket(wsUrl);
      
      ws.onopen = () => {
        setTtyConnected(true);
        setTtyOutput('Terminal connected. Type commands below.\r\n$ ');
        message.success('Terminal connected');
      };
      
      ws.onmessage = (event) => {
        const cleanedData = cleanAnsiEscapes(event.data);
        setTtyOutput(prev => prev + cleanedData);
      };
      
      ws.onclose = () => {
        setTtyConnected(false);
        setTtyOutput(prev => prev + '\r\nTerminal disconnected.');
        message.info('Terminal disconnected');
      };
      
      ws.onerror = (error) => {
        setTtyConnected(false);
        setTtyOutput(prev => prev + '\r\nTerminal error: ' + error.message);
        message.error('Terminal connection failed');
      };
      
      setTtyWs(ws);
    } catch (error) {
      message.error('Failed to connect to terminal');
    }
  };

  const disconnectTTY = () => {
    if (ttyWs) {
      ttyWs.close();
      setTtyWs(null);
    }
  };

  const sendTTYCommand = (command) => {
    if (ttyWs && ttyConnected) {
      ttyWs.send(command);
    }
  };

  const handleTTYKeyPress = (event) => {
    if (event.key === 'Enter') {
      const command = event.target.value + '\r\n';
      sendTTYCommand(command);
      event.target.value = '';
    }
  };

  const downloadPodLogs = () => {
    if (!selectedPod) return;
    
    const downloadUrl = appsAPI.downloadPodLogs(
      id, 
      selectedPod.name, 
      selectedPod.deployment?.deployment_name
    );
    
    // Create a temporary link and trigger download
    const link = document.createElement('a');
    link.href = downloadUrl;
    link.download = `${selectedPod.name}-logs.txt`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const handleDeployModalOk = async () => {
    try {
      const values = await deployForm.validateFields();
      setDeploying(true);
      
      await appsAPI.deployRelease(selectedReleaseId, values);
      message.success('Deployment initiated');
      setDeployModalVisible(false);
      // Refresh data after deployment
      fetchDeployments();
      fetchReleases();
      navigate(`/releases/${selectedReleaseId}/deploy`);
    } catch (error) {
      if (error.errorFields) {
        // Validation failed
        return;
      }
      message.error('Failed to start deployment');
    } finally {
      setDeploying(false);
    }
  };

  const handleDeleteApp = async () => {
    if (!app) return;
    
    setDeleting(true);
    try {
      await appsAPI.deleteApp(app.id);
      message.success(`Application "${app.name}" has been deleted successfully`);
      setDeleteModalVisible(false);
      
      // Refresh the apps list if callback is provided
      if (onAppDeleted) {
        onAppDeleted();
      }
      
      // Navigate back to apps list after successful deletion
      navigate('/apps');
    } catch (error) {
      const errorMessage = error.response?.data?.error || 'Failed to delete application';
      message.error(errorMessage);
      console.error('Delete application error:', error);
    } finally {
      setDeleting(false);
    }
  };

  const handleDeleteDeployment = async () => {
    if (!selectedDeployment) return;
    
    setDeletingDeployment(true);
    try {
      await appsAPI.deleteDeployment(id, selectedDeployment.release_id);
      message.success(`Deployment has been deleted successfully`);
      setDeleteDeploymentModalVisible(false);
      
      // Refresh deployments after deletion
      fetchDeployments();
    } catch (error) {
      const errorMessage = error.response?.data?.error || 'Failed to delete deployment';
      message.error(errorMessage);
      console.error('Delete deployment error:', error);
    } finally {
      setDeletingDeployment(false);
    }
  };


  const getPodStatusColor = (status) => {
    switch (status.toLowerCase()) {
      case 'running':
        return '#52c41a'; // green
      case 'pending':
        return '#faad14'; // yellow
      case 'failed':
      case 'error':
        return '#ff4d4f'; // red
      default:
        return '#d9d9d9'; // gray
    }
  };

  const releasesColumns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: 'Branch',
      dataIndex: 'branch',
      key: 'branch',
      render: (text) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: 'Commit SHA',
      dataIndex: 'commit_sha',
      key: 'commit_sha',
      render: (text) => text ? <code>{text.substring(0, 8)}</code> : '-',
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: getStatusBadge,
    },
    {
      title: 'Image Tag',
      dataIndex: 'image_tag',
      key: 'image_tag',
      render: (text) => text ? <code>{text}</code> : '-',
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text) => new Date(text).toLocaleString(),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => {
        const isBuilding = record.status === 'pending' || record.status === 'running';
        const canDeploy = record.status === 'success';
        const isDeployed = record.status === 'deployed' || record.status === 'deploying';
        
        return (
          <Space size="middle">
            {isBuilding && (
              <Link to={`/releases/${record.id}/build`}>
                <Button type="link" icon={<EyeOutlined />} size="small">
                  View Build
                </Button>
              </Link>
            )}
            {canDeploy && (
              <>
                <Button 
                  type="link" 
                  icon={<PlayCircleOutlined />} 
                  size="small"
                  onClick={() => showDeployModal(record.id)}
                >
                  Upgrade
                </Button>
                <Link to={`/releases/${record.id}/build`}>
                  <Button type="link" icon={<EyeOutlined />} size="small">
                    View Build
                  </Button>
                </Link>
              </>
            )}
            {isDeployed && (
              <Link to={`/releases/${record.id}/deploy`}>
                <Button type="link" icon={<EyeOutlined />} size="small">
                  View Deploy
                </Button>
              </Link>
            )}
            {record.status === 'failed' && (
              <Link to={`/releases/${record.id}/build`}>
                <Button type="link" icon={<ExclamationCircleOutlined />} size="small">
                  View Build
                </Button>
              </Link>
            )}
          </Space>
        );
      },
    },
  ];

  if (loading || !app) {
    return <div>Loading...</div>;
  }

  return (
    <div>
      <Card
        title={
          <Space>
            <Link to="/apps">
              <Button type="text" icon={<ArrowLeftOutlined />} />
            </Link>
            {app.name}
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={fetchApp}>
              Refresh
            </Button>
            <Button 
              type="primary"
              danger
              icon={<DeleteOutlined />} 
              onClick={() => setDeleteModalVisible(true)}
            >
              Delete App
            </Button>
          </Space>
        }
      >
        <Descriptions column={2} style={{ marginBottom: 24 }}>
          <Descriptions.Item label="Git Repository">
            <a href={app.git_url} target="_blank" rel="noopener noreferrer">
              {app.git_url}
            </a>
          </Descriptions.Item>
          <Descriptions.Item label="Default Branch">
            <Tag color="blue">{app.default_branch}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Registry Repository">
            {app.registry_repo}
          </Descriptions.Item>
          <Descriptions.Item label="Target Namespace">
            <Tag color="green">{app.target_namespace}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Dockerfile Path">
            <code>{app.dockerfile_path}</code>
          </Descriptions.Item>
          <Descriptions.Item label="Build Context">
            <code>{app.context_path}</code>
          </Descriptions.Item>
        </Descriptions>

        <Card title="Build" size="small" style={{ marginBottom: 24 }}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Space>
              <Select
                placeholder="Select branch"
                value={selectedBranch?.name}
                onChange={(branchName) => {
                  const branch = branches.find(b => b.name === branchName);
                  setSelectedBranch(branch);
                }}
                loading={branchesLoading}
                onDropdownVisibleChange={(open) => {
                  if (open && branches.length === 0) {
                    fetchBranches();
                  }
                }}
                style={{ width: 350 }}
              >
                {branches.map(branch => (
                  <Option key={branch.name} value={branch.name}>
                    <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                      <span style={{ fontWeight: 500 }}>{branch.name}</span>
                      <span style={{ color: '#999', fontSize: '12px', fontFamily: 'monospace' }}>
                        {branch.sha.substring(0, 7)}
                      </span>
                    </div>
                  </Option>
                ))}
              </Select>
              <Button 
                onClick={fetchBranches}
                loading={branchesLoading}
              >
                Fetch Branches
              </Button>
              <Button 
                onClick={() => validateDockerfile(selectedBranch)}
                loading={validating}
                disabled={!selectedBranch}
                icon={<CheckCircleOutlined />}
              >
                Validate Dockerfile
              </Button>
              <Button 
                type="primary"
                onClick={handleDeploy}
                loading={deploying}
                disabled={!selectedBranch}
                icon={<BuildOutlined />}
              >
                Build
              </Button>
            </Space>

            {selectedBranch && selectedBranch.sha && (
              <Alert
                message={
                  <span>
                    Selected commit: <code style={{ fontFamily: 'monospace' }}>{selectedBranch.sha}</code>
                  </span>
                }
                type="info"
                showIcon
                style={{ marginTop: 8 }}
              />
            )}

            {dockerfileValidation && (
              <Alert
                type={dockerfileValidation.valid ? "success" : "error"}
                message={
                  dockerfileValidation.valid 
                    ? `Dockerfile found at ${dockerfileValidation.path}`
                    : `Dockerfile not found at ${dockerfileValidation.path}`
                }
                description={dockerfileValidation.error}
                showIcon
              />
            )}
          </Space>
        </Card>

        <Card 
          title="Deployed Pods" 
          size="small"
          style={{ marginBottom: 24 }}
          extra={
            <Button icon={<ReloadOutlined />} onClick={fetchDeployments} size="small">
              Refresh
            </Button>
          }
        >
          {deployments.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '20px 0', color: '#999' }}>
              No deployments found. Deploy a successful build to see pods here.
            </div>
          ) : (
            deployments.map((deployment) => (
              <div key={deployment.release_id} style={{ marginBottom: 16 }}>
                {/* Top Section - Deployment Info */}
                <div style={{ 
                  padding: '16px', 
                  background: '#fafafa', 
                  border: '1px solid #d9d9d9',
                  borderRadius: '6px',
                  marginBottom: '12px'
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ display: 'flex', gap: '24px' }}>
                      <div>
                        <strong>Commit SHA:</strong> 
                        <code style={{ marginLeft: '8px' }}>
                          {deployment.commit_sha ? deployment.commit_sha.substring(0, 8) : '-'}
                        </code>
                      </div>
                      <div>
                        <strong>Status:</strong> 
                        <span style={{ marginLeft: '8px' }}>{getStatusBadge(deployment.status)}</span>
                      </div>
                      <div>
                        <strong>Replicas:</strong> 
                        <span style={{ marginLeft: '8px' }}>
                          {deployment.replicas ? `${deployment.replicas.ready}/${deployment.replicas.desired}` : '0/1'}
                        </span>
                      </div>
                      <div>
                        <strong>Deployed:</strong> 
                        <span style={{ marginLeft: '8px' }}>
                          {deployment.deployed_at ? new Date(deployment.deployed_at).toLocaleString() : '-'}
                        </span>
                      </div>
                    </div>
                    <Space size="middle">
                      <Button
                        icon={<ReloadOutlined />}
                        size="small"
                        onClick={() => showReloadModal(deployment)}
                      >
                        Reload
                      </Button>
                      <Button
                        type="primary"
                        icon={<SettingOutlined />}
                        size="small"
                        onClick={() => showScaleModal(deployment)}
                      >
                        Scale
                      </Button>
                      <Link to={`/releases/${deployment.release_id}/deploy`}>
                        <Button type="default" icon={<EyeOutlined />} size="small">
                          View
                        </Button>
                      </Link>
                    </Space>
                  </div>
                </div>

                {/* Bottom Section - Pod Grid */}
                {deployment.pods && deployment.pods.length > 0 && (
                  <div style={{
                    padding: '16px',
                    border: '1px solid #d9d9d9',
                    borderRadius: '6px',
                    background: '#ffffff'
                  }}>
                    {/* Group pods by image tag to identify rolling update status */}
                    {(() => {
                      const podsByVersion = deployment.pods.reduce((acc, pod) => {
                        const version = pod.image_tag || 'unknown';
                        if (!acc[version]) acc[version] = [];
                        acc[version].push(pod);
                        return acc;
                      }, {});
                      
                      const versions = Object.keys(podsByVersion);
                      const isRollingUpdate = versions.length > 1;
                      
                      return (
                        <>
                          <div style={{ marginBottom: '8px', fontWeight: 'bold', fontSize: '13px' }}>
                            Pods ({deployment.pods.length})
                            {isRollingUpdate && (
                              <Tag color="orange" size="small" style={{ marginLeft: '8px' }}>
                                Rolling Update
                              </Tag>
                            )}
                          </div>
                          
                          {versions.map((version, versionIndex) => (
                            <div key={version} style={{ marginBottom: isRollingUpdate ? '12px' : '0px' }}>
                              {isRollingUpdate && (
                                <div style={{ 
                                  marginBottom: '6px', 
                                  fontSize: '11px', 
                                  color: '#666',
                                  fontWeight: 'bold'
                                }}>
                                  {versionIndex === 0 ? '🆕 New Version' : '🔄 Old Version'} ({version})
                                </div>
                              )}
                              
                              <div style={{ 
                                display: 'grid', 
                                gridTemplateColumns: 'repeat(auto-fill, minmax(120px, 1fr))', 
                                gap: '8px',
                                maxHeight: '200px',
                                overflowY: 'auto'
                              }}>
                                {podsByVersion[version].map((pod, index) => (
                                  <div
                                    key={pod.name || index}
                                    style={{
                                      width: '100%',
                                      height: '40px',
                                      backgroundColor: getPodStatusColor(pod.status),
                                      border: isRollingUpdate && versionIndex > 0 
                                        ? `2px dashed ${getPodStatusColor(pod.status)}` 
                                        : `2px solid ${getPodStatusColor(pod.status)}`,
                                      borderRadius: '4px',
                                      display: 'flex',
                                      alignItems: 'center',
                                      justifyContent: 'center',
                                      color: '#fff',
                                      fontSize: '11px',
                                      fontWeight: 'bold',
                                      textShadow: '0 1px 2px rgba(0,0,0,0.3)',
                                      cursor: 'pointer',
                                      transition: 'all 0.2s ease',
                                      opacity: isRollingUpdate && versionIndex > 0 ? 0.7 : 1
                                    }}
                                    title={`Pod: ${pod.name}\nStatus: ${pod.status}\nNode: ${pod.node || 'unknown'}\nRestart Count: ${pod.restart_count || 0}\nImage: ${pod.image || 'unknown'}\nReplicaSet: ${pod.replica_set || 'unknown'}\nClick to view logs`}
                                    onClick={() => showPodLogsModal(pod, deployment)}
                                    onMouseOver={(e) => {
                                      e.target.style.transform = 'scale(1.05)';
                                      e.target.style.boxShadow = '0 2px 8px rgba(0,0,0,0.2)';
                                    }}
                                    onMouseOut={(e) => {
                                      e.target.style.transform = 'scale(1)';
                                      e.target.style.boxShadow = 'none';
                                    }}
                                  >
                                    {pod.status.charAt(0).toUpperCase()}
                                  </div>
                                ))}
                              </div>
                            </div>
                          ))}
                          <div style={{ marginTop: '8px', fontSize: '11px', color: '#666' }}>
                            <span style={{ color: '#52c41a', fontWeight: 'bold' }}>■ Running</span>
                            <span style={{ margin: '0 8px', color: '#faad14', fontWeight: 'bold' }}>■ Pending</span>
                            <span style={{ color: '#ff4d4f', fontWeight: 'bold' }}>■ Failed</span>
                          </div>
                        </>
                      );
                    })()}
                  </div>
                )}
              </div>
            ))
          )}
        </Card>

        {/* Ingress Information Card */}
        <Card 
          title={
            <Space>
              <span>Ingress Information</span>
              <Button 
                type="text" 
                icon={<ReloadOutlined />} 
                size="small" 
                onClick={fetchIngress}
                loading={loadingIngress}
              />
            </Space>
          }
          style={{ marginBottom: '24px' }}
        >
          {loadingIngress ? (
            <div>Loading ingress information...</div>
          ) : ingressInfo && ingressInfo.created ? (
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="Status" span={2}>
                <Badge status="success" text="Active" />
              </Descriptions.Item>
              <Descriptions.Item label="Host">
                {ingressInfo.host || `${app?.name}.local.playquota.com`}
              </Descriptions.Item>
              <Descriptions.Item label="Path">
                <code>{ingressInfo.path || '/'}</code>
              </Descriptions.Item>
              <Descriptions.Item label="Access URL" span={2}>
                <a 
                  href={ingressInfo.url || `http://${app?.name}.local.playquota.com/`}
                  target="_blank" 
                  rel="noopener noreferrer"
                >
                  {ingressInfo.url || `http://${app?.name}.local.playquota.com/`}
                </a>
              </Descriptions.Item>
              {ingressInfo.annotations && (
                <Descriptions.Item label="Annotations" span={2}>
                  <div style={{ fontSize: '12px', fontFamily: 'monospace' }}>
                    {Object.entries(ingressInfo.annotations).map(([key, value]) => (
                      <div key={key}>
                        <strong>{key}:</strong> {value}
                      </div>
                    ))}
                  </div>
                </Descriptions.Item>
              )}
              {ingressInfo.created_at && (
                <Descriptions.Item label="Created" span={2}>
                  {new Date(ingressInfo.created_at).toLocaleString()}
                </Descriptions.Item>
              )}
            </Descriptions>
          ) : app?.ingress_enabled !== false ? (
            <Alert 
              message="Ingress Not Available" 
              description="Ingress will be automatically created after the first successful deployment."
              type="info" 
              showIcon 
            />
          ) : (
            <Alert 
              message="Ingress Disabled" 
              description="Ingress creation is disabled for this application."
              type="warning" 
              showIcon 
            />
          )}
        </Card>

        <Card 
          title="Recent Build" 
          size="small"
          extra={
            <Button icon={<ReloadOutlined />} onClick={fetchReleases} size="small">
              Refresh
            </Button>
          }
        >
          <Table
            columns={releasesColumns}
            dataSource={releases}
            loading={releasesLoading}
            rowKey="id"
            pagination={{ pageSize: 5 }}
            size="small"
          />
        </Card>
      </Card>

      {/* Upgrade Configuration Modal */}
      <Modal
        title={
          <Space>
            <PlayCircleOutlined />
            Upgrade Configuration - Release #{selectedReleaseId}
          </Space>
        }
        visible={deployModalVisible}
        onOk={handleDeployModalOk}
        onCancel={() => setDeployModalVisible(false)}
        width={600}
        okText="Upgrade"
        cancelText="Cancel"
        confirmLoading={deploying}
      >
        <Form
          form={deployForm}
          layout="vertical"
          initialValues={{
            maxUnavailable: 25,
            env_vars: []
          }}
        >
          <Form.Item
            label="Rolling Update Strategy"
            name="maxUnavailable"
            rules={[{ required: true, message: 'Please enter rolling update percentage' }]}
            help="Maximum percentage of pods that can be unavailable during the update (default: 25%)"
          >
            <InputNumber 
              min={1} 
              max={100} 
              style={{ width: '100%' }} 
              formatter={value => `${value}%`}
              parser={value => value.replace('%', '')}
              placeholder="25"
            />
          </Form.Item>

          <Form.Item
            label="Environment Variables"
            help="Configure runtime environment variables for the deployment"
          >
            <Form.List name="env_vars">
              {(fields, { add, remove }) => (
                <>
                  {fields.map(({ key, name, ...restField }) => (
                    <Space
                      key={key}
                      style={{
                        display: 'flex',
                        marginBottom: 8,
                        alignItems: 'flex-start',
                      }}
                      align="baseline"
                    >
                      <Form.Item
                        {...restField}
                        name={[name, 'name']}
                        rules={[
                          { required: true, message: 'Missing variable name' },
                          { pattern: /^[A-Z_][A-Z0-9_]*$/i, message: 'Invalid variable name' }
                        ]}
                        style={{ margin: 0, flex: 1 }}
                      >
                        <Input placeholder="Variable Name" />
                      </Form.Item>
                      <Form.Item
                        {...restField}
                        name={[name, 'value']}
                        rules={[{ required: true, message: 'Missing variable value' }]}
                        style={{ margin: 0, flex: 1 }}
                      >
                        <Input placeholder="Variable Value" />
                      </Form.Item>
                      <Button
                        type="text"
                        icon={<DeleteOutlined />}
                        onClick={() => remove(name)}
                        danger
                        size="small"
                      />
                    </Space>
                  ))}
                  <Form.Item>
                    <Button
                      type="dashed"
                      onClick={() => add()}
                      block
                      icon={<PlusOutlined />}
                    >
                      Add Environment Variable
                    </Button>
                  </Form.Item>
                </>
              )}
            </Form.List>
          </Form.Item>
        </Form>
      </Modal>

      {/* Reload Configuration Modal */}
      <Modal
        title={
          <Space>
            <ReloadOutlined />
            Reload Pods - {selectedReloadDeployment?.deployment_name}
          </Space>
        }
        visible={reloadModalVisible}
        onOk={handleReloadModalOk}
        onCancel={() => setReloadModalVisible(false)}
        width={600}
        okText="Reload"
        cancelText="Cancel"
        confirmLoading={reloading}
      >
        <Form
          form={reloadForm}
          layout="vertical"
          initialValues={{
            maxUnavailable: 25,
            env_vars: []
          }}
        >
          <Form.Item
            label="Rolling Update Strategy"
            name="maxUnavailable"
            rules={[{ required: true, message: 'Please enter rolling update percentage' }]}
            help="Maximum percentage of pods that can be unavailable during the restart (default: 25%)"
          >
            <InputNumber
              min={1}
              max={100}
              style={{ width: '100%' }}
              formatter={value => `${value}%`}
              parser={value => value.replace('%', '')}
              placeholder="25"
            />
          </Form.Item>

          <Form.Item
            label="Environment Variables"
            help="Configure runtime environment variables for the deployment"
          >
            <Form.List name="env_vars">
              {(fields, { add, remove }) => (
                <>
                  {fields.map(({ key, name, ...restField }) => (
                    <Space
                      key={key}
                      style={{
                        display: 'flex',
                        marginBottom: 8,
                        alignItems: 'flex-start',
                      }}
                      align="baseline"
                    >
                      <Form.Item
                        {...restField}
                        name={[name, 'name']}
                        rules={[
                          { required: true, message: 'Missing variable name' },
                          { pattern: /^[A-Z_][A-Z0-9_]*$/i, message: 'Invalid variable name' }
                        ]}
                        style={{ margin: 0, flex: 1 }}
                      >
                        <Input placeholder="Variable Name" />
                      </Form.Item>
                      <Form.Item
                        {...restField}
                        name={[name, 'value']}
                        rules={[{ required: true, message: 'Missing variable value' }]}
                        style={{ margin: 0, flex: 1 }}
                      >
                        <Input placeholder="Variable Value" />
                      </Form.Item>
                      <Button
                        type="text"
                        icon={<DeleteOutlined />}
                        onClick={() => remove(name)}
                        danger
                        size="small"
                      />
                    </Space>
                  ))}
                  <Form.Item>
                    <Button
                      type="dashed"
                      onClick={() => add()}
                      block
                      icon={<PlusOutlined />}
                    >
                      Add Environment Variable
                    </Button>
                  </Form.Item>
                </>
              )}
            </Form.List>
          </Form.Item>
        </Form>
      </Modal>

      {/* Scale Configuration Modal */}
      <Modal
        title={
          <Space>
            <SettingOutlined />
            Scale Deployment - {selectedDeployment?.deployment_name}
          </Space>
        }
        visible={scaleModalVisible}
        onOk={handleScaleModalOk}
        onCancel={() => setScaleModalVisible(false)}
        width={400}
        okText="Scale"
        cancelText="Cancel"
        confirmLoading={scaling}
      >
        <Form
          form={scaleForm}
          layout="vertical"
          initialValues={{
            replicas: 1
          }}
        >
          <Form.Item
            label="Number of Replicas"
            name="replicas"
            rules={[{ required: true, message: 'Please enter number of replicas' }]}
            help="Set the number of pod replicas to run"
          >
            <InputNumber 
              min={0} 
              max={100} 
              style={{ width: '100%' }} 
              placeholder="Enter desired replica count"
            />
          </Form.Item>
          
          {selectedDeployment && (
            <Alert
              message="Current Deployment Status"
              description={
                <div>
                  <div>Release: #{selectedDeployment.release_id}</div>
                  <div>Image: <code>{selectedDeployment.image_tag}</code></div>
                  <div>Current Replicas: {selectedDeployment.replicas?.ready || 0}/{selectedDeployment.replicas?.desired || 1}</div>
                </div>
              }
              type="info"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}
        </Form>
      </Modal>

      {/* Pod Logs Modal */}
      <Modal
        title={
          <Space>
            <FileTextOutlined />
            Pod Details - {selectedPod?.name}
          </Space>
        }
        visible={podLogsModalVisible}
        onOk={() => {
          disconnectTTY();
          setPodLogsModalVisible(false);
          setPodDescribe(null); // Reset pod describe data
          setActiveTab('logs'); // Reset to default tab
        }}
        onCancel={() => {
          disconnectTTY();
          setPodLogsModalVisible(false);
          setPodDescribe(null); // Reset pod describe data
          setActiveTab('logs'); // Reset to default tab
        }}
        width={1000}
        okText="Close"
        cancelButtonProps={{ style: { display: 'none' } }}
        footer={[
          activeTab === 'logs' && (
            <Button 
              key="download" 
              icon={<DownloadOutlined />}
              onClick={downloadPodLogs}
              disabled={loadingLogs || !podLogs || podLogs.startsWith('Error')}
            >
              Download Logs
            </Button>
          ),
          activeTab === 'terminal' && (
            <Button 
              key="connect" 
              type={ttyConnected ? 'danger' : 'primary'}
              icon={<ConsoleSqlOutlined />}
              onClick={ttyConnected ? disconnectTTY : connectTTY}
            >
              {ttyConnected ? 'Disconnect' : 'Connect Terminal'}
            </Button>
          ),
          <Button key="close" onClick={() => {
            disconnectTTY();
            setPodLogsModalVisible(false);
            setPodDescribe(null); // Reset pod describe data
            setActiveTab('logs'); // Reset to default tab
          }}>
            Close
          </Button>
        ].filter(Boolean)}
      >
        {selectedPod && (
          <div style={{ marginBottom: 16 }}>
            <Alert
              message="Pod Information"
              description={
                <div>
                  <div><strong>Pod Name:</strong> <code>{selectedPod.name}</code></div>
                  <div><strong>Status:</strong> <Badge status={selectedPod.status === 'Running' ? 'success' : selectedPod.status === 'Pending' ? 'processing' : 'error'} text={selectedPod.status} /></div>
                  <div><strong>Node:</strong> {selectedPod.node || 'unknown'}</div>
                  <div><strong>Restart Count:</strong> {selectedPod.restart_count || 0}</div>
                  {selectedPod.started_at && <div><strong>Started:</strong> {new Date(selectedPod.started_at).toLocaleString()}</div>}
                </div>
              }
              type="info"
              showIcon
            />
          </div>
        )}

        <Tabs 
          activeKey={activeTab} 
          onChange={(key) => {
            setActiveTab(key);
            if (key === 'describe' && selectedPod) {
              fetchPodDescribe();
            }
          }}
          items={[
            {
              key: 'logs',
              label: (
                <span>
                  <FileTextOutlined />
                  Logs
                </span>
              ),
              children: (
                <div style={{ height: '500px', border: '1px solid #d9d9d9', borderRadius: '4px' }}>
                  {loadingLogs ? (
                    <div style={{ 
                      display: 'flex', 
                      alignItems: 'center', 
                      justifyContent: 'center', 
                      height: '100%',
                      background: '#fafafa' 
                    }}>
                      <div style={{ textAlign: 'center' }}>
                        <div style={{ marginBottom: 8 }}>Loading logs...</div>
                        <div>Please wait...</div>
                      </div>
                    </div>
                  ) : (
                    <div
                      style={{
                        height: '100%',
                        padding: '12px',
                        backgroundColor: '#1e1e1e',
                        color: '#ffffff',
                        fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
                        fontSize: '12px',
                        lineHeight: '1.4',
                        overflow: 'auto',
                        whiteSpace: 'pre-wrap',
                        wordBreak: 'break-all'
                      }}
                    >
                      {podLogs || 'No logs available'}
                    </div>
                  )}
                </div>
              )
            },
            {
              key: 'terminal',
              label: (
                <span>
                  <ConsoleSqlOutlined />
                  Terminal
                </span>
              ),
              children: (
                <div style={{ height: '500px' }}>
                  <div style={{ 
                    height: '460px', 
                    border: '1px solid #d9d9d9', 
                    borderRadius: '4px 4px 0 0',
                    backgroundColor: '#1e1e1e',
                    color: '#00ff00',
                    fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
                    fontSize: '12px',
                    padding: '8px',
                    overflow: 'auto',
                    whiteSpace: 'pre-wrap'
                  }}>
                    {ttyOutput || (ttyConnected ? 'Terminal ready...' : 'Click "Connect Terminal" to start a terminal session')}
                  </div>
                  <Input 
                    placeholder="Type command and press Enter..."
                    onKeyPress={handleTTYKeyPress}
                    disabled={!ttyConnected}
                    style={{ 
                      borderRadius: '0 0 4px 4px',
                      fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace'
                    }}
                  />
                </div>
              )
            },
            {
              key: 'describe',
              label: (
                <span>
                  <EyeOutlined />
                  Describe
                </span>
              ),
              children: (
                <div style={{ height: '500px', border: '1px solid #d9d9d9', borderRadius: '4px', overflow: 'auto' }}>
                  {loadingDescribe ? (
                    <div style={{ 
                      display: 'flex', 
                      alignItems: 'center', 
                      justifyContent: 'center', 
                      height: '100%',
                      background: '#fafafa' 
                    }}>
                      <div style={{ textAlign: 'center' }}>
                        <div style={{ marginBottom: 8 }}>Loading pod details...</div>
                        <div>Please wait...</div>
                      </div>
                    </div>
                  ) : podDescribe?.error ? (
                    <div style={{ 
                      padding: '16px', 
                      color: '#ff4d4f',
                      fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
                      fontSize: '12px'
                    }}>
                      {podDescribe.error}
                    </div>
                  ) : podDescribe ? (
                    <div style={{ 
                      padding: '16px', 
                      fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
                      fontSize: '12px',
                      lineHeight: '1.4'
                    }}>
                      {/* Basic Information */}
                      <div style={{ marginBottom: 16 }}>
                        <div><strong>Name:</strong> {podDescribe.name}</div>
                        <div><strong>Namespace:</strong> {podDescribe.namespace}</div>
                        <div><strong>Priority:</strong> {podDescribe.priority || '0'}</div>
                        <div><strong>Node:</strong> {podDescribe.node || '<none>'}</div>
                        <div><strong>Start Time:</strong> {podDescribe.start_time ? new Date(podDescribe.start_time).toLocaleString() : '<unknown>'}</div>
                        <div><strong>Labels:</strong></div>
                        {podDescribe.labels && Object.keys(podDescribe.labels).length > 0 ? (
                          <div style={{ marginLeft: 20 }}>
                            {Object.entries(podDescribe.labels).map(([key, value]) => (
                              <div key={key}>{key}={value}</div>
                            ))}
                          </div>
                        ) : (
                          <div style={{ marginLeft: 20 }}>{'<none>'}</div>
                        )}
                        <div><strong>Annotations:</strong></div>
                        {podDescribe.annotations && Object.keys(podDescribe.annotations).length > 0 ? (
                          <div style={{ marginLeft: 20 }}>
                            {Object.entries(podDescribe.annotations).map(([key, value]) => (
                              <div key={key} style={{ wordBreak: 'break-all' }}>{key}: {value}</div>
                            ))}
                          </div>
                        ) : (
                          <div style={{ marginLeft: 20 }}>{'<none>'}</div>
                        )}
                      </div>

                      {/* Status */}
                      <div style={{ marginBottom: 16 }}>
                        <div><strong>Status:</strong> {podDescribe.status?.phase || 'Unknown'}</div>
                        <div><strong>IP:</strong> {podDescribe.status?.ip || '<none>'}</div>
                        {podDescribe.status?.ips && podDescribe.status.ips.length > 0 && (
                          <div><strong>IPs:</strong></div>
                        )}
                        {podDescribe.status?.ips?.map((ip, index) => (
                          <div key={index} style={{ marginLeft: 20 }}>
                            IP: {ip.ip}
                          </div>
                        ))}
                        <div><strong>Controlled By:</strong></div>
                        {podDescribe.controlled_by && podDescribe.controlled_by.length > 0 ? (
                          podDescribe.controlled_by.map((owner, index) => (
                            <div key={index} style={{ marginLeft: 20 }}>
                              {owner.kind}/{owner.name}
                            </div>
                          ))
                        ) : (
                          <div style={{ marginLeft: 20 }}>{'<none>'}</div>
                        )}
                      </div>

                      {/* Init Containers */}
                      {podDescribe.init_containers && podDescribe.init_containers.length > 0 && (
                        <div style={{ marginBottom: 16 }}>
                          <div><strong>Init Containers:</strong></div>
                          {podDescribe.init_containers.map((container, index) => (
                            <div key={index} style={{ marginLeft: 20, marginBottom: 8 }}>
                              <div>{container.name}:</div>
                              <div style={{ marginLeft: 20 }}>
                                <div>Image: {container.image}</div>
                                {container.ports && container.ports.length > 0 && (
                                  <div>Ports: {container.ports.map(p => `${p.containerPort}/${p.protocol || 'TCP'}`).join(', ')}</div>
                                )}
                                {container.requests && (
                                  <div>Requests: {Object.entries(container.requests).map(([k, v]) => `${k}: ${v}`).join(', ')}</div>
                                )}
                                {container.limits && (
                                  <div>Limits: {Object.entries(container.limits).map(([k, v]) => `${k}: ${v}`).join(', ')}</div>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Containers */}
                      {podDescribe.containers && podDescribe.containers.length > 0 && (
                        <div style={{ marginBottom: 16 }}>
                          <div><strong>Containers:</strong></div>
                          {podDescribe.containers.map((container, index) => (
                            <div key={index} style={{ marginLeft: 20, marginBottom: 12 }}>
                              <div>{container.name}:</div>
                              <div style={{ marginLeft: 20 }}>
                                <div>Image: {container.image}</div>
                                <div>Image ID: {podDescribe.container_statuses?.[index]?.image_id || '<unknown>'}</div>
                                {container.ports && container.ports.length > 0 && (
                                  <div>Ports: {container.ports.map(p => `${p.containerPort}/${p.protocol || 'TCP'}`).join(', ')}</div>
                                )}
                                <div>Host Network: {container.host_network ? 'true' : 'false'}</div>
                                {container.requests && (
                                  <div>Requests: {Object.entries(container.requests).map(([k, v]) => `${k}: ${v}`).join(', ')}</div>
                                )}
                                {container.limits && (
                                  <div>Limits: {Object.entries(container.limits).map(([k, v]) => `${k}: ${v}`).join(', ')}</div>
                                )}
                                
                                {/* Container Status */}
                                {podDescribe.container_statuses?.[index] && (
                                  <>
                                    <div>State: </div>
                                    <div style={{ marginLeft: 20 }}>
                                      {podDescribe.container_statuses[index].state?.running && (
                                        <div>Running</div>
                                      )}
                                      {podDescribe.container_statuses[index].state?.waiting && (
                                        <div>
                                          Waiting
                                          <div style={{ marginLeft: 20 }}>
                                            Reason: {podDescribe.container_statuses[index].state.waiting.reason}
                                          </div>
                                        </div>
                                      )}
                                      {podDescribe.container_statuses[index].state?.terminated && (
                                        <div>
                                          Terminated
                                          <div style={{ marginLeft: 20 }}>
                                            Reason: {podDescribe.container_statuses[index].state.terminated.reason}<br/>
                                            Exit Code: {podDescribe.container_statuses[index].state.terminated.exit_code}<br/>
                                            Started: {podDescribe.container_statuses[index].state.terminated.started_at ? 
                                              new Date(podDescribe.container_statuses[index].state.terminated.started_at).toLocaleString() : '<unknown>'}<br/>
                                            Finished: {podDescribe.container_statuses[index].state.terminated.finished_at ? 
                                              new Date(podDescribe.container_statuses[index].state.terminated.finished_at).toLocaleString() : '<unknown>'}
                                          </div>
                                        </div>
                                      )}
                                    </div>
                                    <div>Ready: {podDescribe.container_statuses[index].ready ? 'True' : 'False'}</div>
                                    <div>Restart Count: {podDescribe.container_statuses[index].restart_count || 0}</div>
                                  </>
                                )}

                                {/* Environment Variables */}
                                {container.environment && container.environment.length > 0 && (
                                  <>
                                    <div>Environment:</div>
                                    <div style={{ marginLeft: 20 }}>
                                      {container.environment.map((env, envIndex) => (
                                        <div key={envIndex}>
                                          {env.name}: {env.value || (env.value_from ? '<set from secret/configmap>' : '<undefined>')}
                                        </div>
                                      ))}
                                    </div>
                                  </>
                                )}

                                {/* Mounts */}
                                {container.mounts && container.mounts.length > 0 && (
                                  <>
                                    <div>Mounts:</div>
                                    <div style={{ marginLeft: 20 }}>
                                      {container.mounts.map((mount, mountIndex) => (
                                        <div key={mountIndex}>
                                          {mount.mount_path} from {mount.name} ({mount.read_only ? 'ro' : 'rw'})
                                        </div>
                                      ))}
                                    </div>
                                  </>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Conditions */}
                      {podDescribe.conditions && podDescribe.conditions.length > 0 && (
                        <div style={{ marginBottom: 16 }}>
                          <div><strong>Conditions:</strong></div>
                          <div style={{ marginLeft: 20 }}>
                            <div>Type    Status  LastProbeTime    LastTransitionTime    Reason</div>
                            <div>----    ------  -------------    ------------------    ------</div>
                            {podDescribe.conditions.map((condition, index) => (
                              <div key={index}>
                                {condition.type}    {condition.status}    {condition.last_probe_time || '<unknown>'}    {condition.last_transition_time ? new Date(condition.last_transition_time).toLocaleString() : '<unknown>'}    {condition.reason || ''}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Volumes */}
                      {podDescribe.volumes && podDescribe.volumes.length > 0 && (
                        <div style={{ marginBottom: 16 }}>
                          <div><strong>Volumes:</strong></div>
                          {podDescribe.volumes.map((volume, index) => (
                            <div key={index} style={{ marginLeft: 20, marginBottom: 4 }}>
                              <div>{volume.name}:</div>
                              <div style={{ marginLeft: 20 }}>
                                <div>Type: {volume.type}</div>
                                {volume.type === 'PVC' && (
                                  <div>ClaimName: {volume.claim_name}</div>
                                )}
                                {volume.type === 'HostPath' && volume.host_path && (
                                  <>
                                    <div>Path: {volume.host_path.path}</div>
                                    <div>HostPathType: {volume.host_path.type || 'File'}</div>
                                  </>
                                )}
                                {volume.type === 'Secret' && volume.secret && (
                                  <div>SecretName: {volume.secret.secretName}</div>
                                )}
                                {volume.type === 'ConfigMap' && volume.config_map && (
                                  <div>Name: {volume.config_map.name}</div>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}

                      {/* QoS Class */}
                      <div style={{ marginBottom: 16 }}>
                        <div><strong>QoS Class:</strong> {podDescribe.qos_class || 'BestEffort'}</div>
                      </div>

                      {/* Node-Selectors */}
                      <div style={{ marginBottom: 16 }}>
                        <div><strong>Node-Selectors:</strong></div>
                        {podDescribe.node_selectors && Object.keys(podDescribe.node_selectors).length > 0 ? (
                          <div style={{ marginLeft: 20 }}>
                            {Object.entries(podDescribe.node_selectors).map(([key, value]) => (
                              <div key={key}>{key}={value}</div>
                            ))}
                          </div>
                        ) : (
                          <div style={{ marginLeft: 20 }}>{'<none>'}</div>
                        )}
                      </div>

                      {/* Tolerations */}
                      <div style={{ marginBottom: 16 }}>
                        <div><strong>Tolerations:</strong></div>
                        {podDescribe.tolerations && podDescribe.tolerations.length > 0 ? (
                          <div style={{ marginLeft: 20 }}>
                            {podDescribe.tolerations.map((toleration, index) => (
                              <div key={index}>
                                {toleration.key || '<none>'}:{toleration.effect || 'NoSchedule'} for {toleration.tolerationSeconds || 'infinite'}s
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div style={{ marginLeft: 20 }}>{'<none>'}</div>
                        )}
                      </div>

                      {/* Events */}
                      {podDescribe.events && podDescribe.events.length > 0 ? (
                        <div>
                          <div><strong>Events:</strong></div>
                          <div style={{ marginLeft: 20, fontFamily: 'monospace' }}>
                            <div style={{ borderBottom: '1px solid #ddd', paddingBottom: 4, marginBottom: 4 }}>
                              <span style={{ display: 'inline-block', width: '80px' }}>Type</span>
                              <span style={{ display: 'inline-block', width: '120px' }}>Reason</span>
                              <span style={{ display: 'inline-block', width: '80px' }}>Age</span>
                              <span style={{ display: 'inline-block', width: '100px' }}>From</span>
                              <span style={{ display: 'inline-block', width: '120px' }}>Object</span>
                              <span>Message</span>
                            </div>
                            {podDescribe.events.map((event, index) => (
                              <div key={index} style={{ marginBottom: 2, fontSize: '11px' }}>
                                <span style={{ 
                                  display: 'inline-block', 
                                  width: '80px',
                                  color: event.type === 'Warning' ? '#ff4d4f' : '#52c41a'
                                }}>
                                  {event.type}
                                </span>
                                <span style={{ display: 'inline-block', width: '120px' }}>
                                  {event.reason}
                                </span>
                                <span style={{ display: 'inline-block', width: '80px' }}>
                                  {event.age}
                                </span>
                                <span style={{ display: 'inline-block', width: '100px' }}>
                                  {event.from}
                                </span>
                                <span style={{ display: 'inline-block', width: '120px' }}>
                                  {event.object}
                                </span>
                                <span style={{ color: '#666' }}>
                                  {event.message}
                                </span>
                              </div>
                            ))}
                          </div>
                        </div>
                      ) : (
                        <div>
                          <div><strong>Events:</strong></div>
                          <div style={{ marginLeft: 20, color: '#999', fontStyle: 'italic' }}>
                            No events found for this pod
                          </div>
                        </div>
                      )}
                    </div>
                  ) : (
                    <div style={{ 
                      display: 'flex', 
                      alignItems: 'center', 
                      justifyContent: 'center', 
                      height: '100%',
                      color: '#999',
                      background: '#fafafa'
                    }}>
                      No pod description available
                    </div>
                  )}
                </div>
              )
            }
          ]}
        />
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        title={
          <Space>
            <ExclamationCircleOutlined style={{ color: '#ff4d4f' }} />
            Delete Application
          </Space>
        }
        visible={deleteModalVisible}
        onOk={handleDeleteApp}
        onCancel={() => setDeleteModalVisible(false)}
        width={500}
        okText="Delete"
        cancelText="Cancel"
        confirmLoading={deleting}
        okButtonProps={{ danger: true }}
      >
        <div>
          <Alert
            message="Warning: This action cannot be undone"
            description="Deleting this application will permanently remove all builds, releases, logs, and associated images from Harbor registry."
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
          />
          
          <div style={{ marginBottom: 16 }}>
            <strong>Application to delete:</strong>
            <div style={{ marginTop: 8, padding: '8px 12px', backgroundColor: '#f5f5f5', border: '1px solid #d9d9d9', borderRadius: '4px' }}>
              <div><strong>Name:</strong> {app?.name}</div>
              <div><strong>Repository:</strong> {app?.git_url}</div>
              <div><strong>Registry:</strong> {app?.registry_repo}</div>
            </div>
          </div>
          
          {deployments.length > 0 && (
            <Alert
              message="Active Deployments Detected"
              description="This application has running pods. All pods will be terminated before deletion."
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
            />
          )}
          
          <div>
            <strong>This will delete:</strong>
            <ul style={{ margin: '8px 0', paddingLeft: '20px' }}>
              <li>Application configuration</li>
              <li>All build releases and history</li>
              <li>All build and deployment logs</li>
              <li>All Docker images in Harbor registry</li>
              <li>Running pods and deployments</li>
            </ul>
          </div>
        </div>
      </Modal>

      {/* Delete Deployment Confirmation Modal */}
      <Modal
        title={
          <Space>
            <ExclamationCircleOutlined style={{ color: '#ff4d4f' }} />
            Delete Deployment
          </Space>
        }
        visible={deleteDeploymentModalVisible}
        onOk={handleDeleteDeployment}
        onCancel={() => setDeleteDeploymentModalVisible(false)}
        width={500}
        okText="Delete Deployment"
        cancelText="Cancel"
        confirmLoading={deletingDeployment}
        okButtonProps={{ danger: true }}
      >
        <div>
          <Alert
            message="Warning: This action cannot be undone"
            description="Deleting this deployment will permanently remove all pods and the Kubernetes deployment."
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
          />
          
          <div style={{ marginBottom: 16 }}>
            <strong>Deployment to delete:</strong>
            <div style={{ marginTop: 8, padding: '8px 12px', backgroundColor: '#f5f5f5', border: '1px solid #d9d9d9', borderRadius: '4px' }}>
              <div><strong>Release ID:</strong> #{selectedDeployment?.release_id}</div>
              <div><strong>Commit SHA:</strong> {selectedDeployment?.commit_sha?.substring(0, 8) || '-'}</div>
              <div><strong>Image:</strong> {selectedDeployment?.image_tag}</div>
              <div><strong>Current Replicas:</strong> {selectedDeployment?.replicas?.ready || 0}/{selectedDeployment?.replicas?.desired || 1}</div>
            </div>
          </div>
          
          <div>
            <strong>This will:</strong>
            <ul style={{ margin: '8px 0', paddingLeft: '20px' }}>
              <li>Delete all running pods</li>
              <li>Remove the Kubernetes deployment</li>
              <li>Stop all traffic to this application version</li>
              <li>Cannot be reversed - you'll need to redeploy</li>
            </ul>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default AppDetails;