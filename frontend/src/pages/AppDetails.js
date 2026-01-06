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
  InputNumber
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
  DeleteOutlined
} from '@ant-design/icons';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';

const { Option } = Select;

const AppDetails = () => {
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
  const [deploymentsLoading, setDeploymentsLoading] = useState(false);
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [validating, setValidating] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [deployModalVisible, setDeployModalVisible] = useState(false);
  const [selectedReleaseId, setSelectedReleaseId] = useState(null);
  const [deployForm] = Form.useForm();

  useEffect(() => {
    if (id) {
      fetchApp();
      fetchReleases();
      fetchDeployments();
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
      // Filter to show only build-related releases (pending, running, success, failed)
      const buildReleases = response.data.releases?.filter(release => 
        ['pending', 'running', 'success', 'failed'].includes(release.status)
      ) || [];
      setReleases(buildReleases);
    } catch (error) {
      message.error('Failed to fetch releases');
    } finally {
      setReleasesLoading(false);
    }
  };

  const fetchDeployments = async () => {
    setDeploymentsLoading(true);
    try {
      const response = await appsAPI.getAppDeployments(id);
      setDeployments(response.data.deployments || []);
    } catch (error) {
      message.error('Failed to fetch deployments');
    } finally {
      setDeploymentsLoading(false);
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
      replicas: 1,
      env_vars: []
    });
    setDeployModalVisible(true);
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

  const deploymentColumns = [
    {
      title: 'Release',
      dataIndex: 'release_id',
      key: 'release_id',
      width: 80,
      render: (text) => `#${text}`,
    },
    {
      title: 'Image Tag',
      dataIndex: 'image_tag',
      key: 'image_tag',
      render: (text) => text ? <code>{text}</code> : '-',
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
      render: (text) => getStatusBadge(text),
    },
    {
      title: 'Replicas',
      dataIndex: 'replicas',
      key: 'replicas',
      render: (replicas) => replicas ? `${replicas.ready}/${replicas.desired}` : '0/1',
    },
    {
      title: 'Deployed At',
      dataIndex: 'deployed_at',
      key: 'deployed_at',
      render: (text) => text ? new Date(text).toLocaleString() : '-',
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <Space size="middle">
          <Link to={`/releases/${record.release_id}/deploy`}>
            <Button type="link" icon={<EyeOutlined />} size="small">
              View
            </Button>
          </Link>
        </Space>
      ),
    },
  ];

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
                  Deploy
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
            <Table
              columns={deploymentColumns}
              dataSource={deployments}
              loading={deploymentsLoading}
              rowKey="release_id"
              pagination={false}
              size="small"
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

      {/* Deploy Configuration Modal */}
      <Modal
        title={
          <Space>
            <SettingOutlined />
            Deploy Configuration - Release #{selectedReleaseId}
          </Space>
        }
        visible={deployModalVisible}
        onOk={handleDeployModalOk}
        onCancel={() => setDeployModalVisible(false)}
        width={600}
        okText="Deploy"
        cancelText="Cancel"
        confirmLoading={deploying}
      >
        <Form
          form={deployForm}
          layout="vertical"
          initialValues={{
            replicas: 1,
            env_vars: []
          }}
        >
          <Form.Item
            label="Replicas"
            name="replicas"
            rules={[{ required: true, message: 'Please enter number of replicas' }]}
            help="Number of pod replicas to run"
          >
            <InputNumber min={1} max={100} style={{ width: '100%' }} />
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
    </div>
  );
};

export default AppDetails;