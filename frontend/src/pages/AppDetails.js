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
  Badge
} from 'antd';
import { 
  ArrowLeftOutlined, 
  ReloadOutlined, 
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  PlayCircleOutlined,
  EyeOutlined 
} from '@ant-design/icons';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';

const { Option } = Select;

const AppDetails = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [app, setApp] = useState(null);
  const [releases, setReleases] = useState([]);
  const [branches, setBranches] = useState([]);
  const [selectedBranch, setSelectedBranch] = useState('');
  const [dockerfileValidation, setDockerfileValidation] = useState(null);
  const [loading, setLoading] = useState(false);
  const [releasesLoading, setReleasesLoading] = useState(false);
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [validating, setValidating] = useState(false);
  const [deploying, setDeploying] = useState(false);

  useEffect(() => {
    if (id) {
      fetchApp();
      fetchReleases();
    }
  }, [id]);

  const fetchApp = async () => {
    setLoading(true);
    try {
      const response = await appsAPI.getApp(id);
      setApp(response.data);
      setSelectedBranch(response.data.default_branch);
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
      setReleases(response.data.releases || []);
    } catch (error) {
      message.error('Failed to fetch releases');
    } finally {
      setReleasesLoading(false);
    }
  };

  const fetchBranches = async () => {
    setBranchesLoading(true);
    try {
      const response = await appsAPI.getAppBranches(id);
      setBranches(response.data.branches || []);
    } catch (error) {
      message.error('Failed to fetch branches');
    } finally {
      setBranchesLoading(false);
    }
  };

  const validateDockerfile = async (branch) => {
    setValidating(true);
    try {
      const response = await appsAPI.validateDockerfile(id, branch);
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
    if (!dockerfileValidation || !dockerfileValidation.valid) {
      Modal.confirm({
        title: 'Dockerfile Not Validated',
        content: 'Dockerfile has not been validated for this branch. Do you want to validate it first?',
        onOk: () => validateDockerfile(selectedBranch),
      });
      return;
    }

    Modal.confirm({
      title: 'Deploy Application',
      content: `Are you sure you want to deploy branch "${selectedBranch}"?`,
      onOk: async () => {
        setDeploying(true);
        try {
          const response = await appsAPI.createRelease(id, {
            branch: selectedBranch,
          });
          message.success('Release created successfully');
          navigate(`/releases/${response.data.id}`);
        } catch (error) {
          message.error('Failed to create release');
        } finally {
          setDeploying(false);
        }
      },
    });
  };

  const getStatusBadge = (status) => {
    const statusConfig = {
      pending: { status: 'default', text: 'Pending' },
      running: { status: 'processing', text: 'Running' },
      success: { status: 'success', text: 'Success' },
      failed: { status: 'error', text: 'Failed' },
      canceled: { status: 'default', text: 'Canceled' },
    };
    const config = statusConfig[status] || { status: 'default', text: status };
    return <Badge status={config.status} text={config.text} />;
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
      render: (_, record) => (
        <Space size="middle">
          <Link to={`/releases/${record.id}`}>
            <Button type="link" icon={<EyeOutlined />} size="small">
              View
            </Button>
          </Link>
        </Space>
      ),
    },
  ];

  if (loading || !app) {
    return <div style={{ padding: 24 }}>Loading...</div>;
  }

  return (
    <div style={{ padding: 24 }}>
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
          <Button icon={<ReloadOutlined />} onClick={fetchApp}>
            Refresh
          </Button>
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

        <Card title="Deploy" size="small" style={{ marginBottom: 24 }}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Space>
              <Select
                placeholder="Select branch"
                value={selectedBranch}
                onChange={setSelectedBranch}
                loading={branchesLoading}
                onDropdownVisibleChange={(open) => {
                  if (open && branches.length === 0) {
                    fetchBranches();
                  }
                }}
                style={{ width: 200 }}
              >
                {branches.map(branch => (
                  <Option key={branch} value={branch}>{branch}</Option>
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
                icon={<PlayCircleOutlined />}
              >
                Deploy
              </Button>
            </Space>

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
          title="Recent Releases" 
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
    </div>
  );
};

export default AppDetails;