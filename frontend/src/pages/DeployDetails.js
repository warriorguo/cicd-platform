import React, { useState, useEffect, useRef } from 'react';
import { 
  Card, 
  Button, 
  Steps, 
  Alert, 
  Typography, 
  Space, 
  Tag,
  Descriptions,
  message,
  Spin,
  Modal,
  Form,
  Input,
  InputNumber
} from 'antd';
import { 
  ArrowLeftOutlined, 
  ReloadOutlined,
  PlayCircleOutlined,
  RollbackOutlined,
  BuildOutlined,
  PlusOutlined,
  DeleteOutlined,
  SettingOutlined
} from '@ant-design/icons';
import { Link, useParams } from 'react-router-dom';
import { appsAPI, createWebSocketConnection } from '../services/api';
import { getStatusBadge } from '../utils/statusHelpers';

const { Step } = Steps;
const { Text } = Typography;

const DeployDetails = () => {
  const { id } = useParams();
  const [release, setRelease] = useState(null);
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [wsConnection, setWsConnection] = useState(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [deployModalVisible, setDeployModalVisible] = useState(false);
  const [deployForm] = Form.useForm();
  const intervalRef = useRef(null);

  useEffect(() => {
    if (id) {
      fetchRelease();
      setupWebSocket();
      setupAutoRefresh();
    }

    return () => {
      if (wsConnection) {
        wsConnection.close();
      }
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [id]);

  const setupAutoRefresh = () => {
    // Set up auto refresh every 5 seconds when deployment is running
    intervalRef.current = setInterval(() => {
      if (autoRefresh && release && 
          (release.status === 'deploying' || release.status === 'running')) {
        fetchRelease(false);
      }
    }, 5000);
  };

  const fetchRelease = async (showLoading = true) => {
    if (showLoading) setLoading(true);
    try {
      const response = await appsAPI.getRelease(id);
      setRelease(response.data);

      // Stop auto refresh if deployment is completed
      if (response.data.status === 'deployed' || 
          response.data.status === 'failed') {
        setAutoRefresh(false);
      }
    } catch (error) {
      message.error('Failed to fetch release');
    } finally {
      if (showLoading) setLoading(false);
    }
  };

  const setupWebSocket = () => {
    try {
      const ws = createWebSocketConnection(id);
      
      ws.onopen = () => {
        // WebSocket connected
      };

      ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        
        if (data.type === 'status') {
          setRelease(prev => ({
            ...prev,
            status: data.payload.status,
            started_at: data.payload.started_at,
            finished_at: data.payload.finished_at,
          }));
        } else if (data.type === 'log') {
          setLogs(prev => [...prev, data.payload]);
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      ws.onclose = () => {
        // WebSocket disconnected
      };

      setWsConnection(ws);
    } catch (error) {
      console.error('Failed to setup WebSocket:', error);
    }
  };

  const showDeployModal = () => {
    // Initialize form with default values
    deployForm.setFieldsValue({
      replicas: 1,
      env_vars: []
    });
    setDeployModalVisible(true);
  };

  const handleDeploy = async (deployParams) => {
    try {
      await appsAPI.deployRelease(id, deployParams);
      message.success('Deployment initiated');
      setDeployModalVisible(false);
      setAutoRefresh(true);
    } catch (error) {
      message.error('Failed to start deployment');
    }
  };

  const handleDeployModalOk = async () => {
    try {
      const values = await deployForm.validateFields();
      await handleDeploy(values);
    } catch (error) {
      // Validation failed
    }
  };

  const handleRollback = async () => {
    try {
      await appsAPI.rollbackRelease(id);
      message.success('Rollback initiated');
    } catch (error) {
      message.error('Failed to rollback release');
    }
  };


  const getStepStatus = (stepName, releaseStatus) => {
    if (!release) return 'wait';
    
    
    // Build is always complete if we're on this page
    if (stepName === 'build-complete') return 'finish';
    
    switch (releaseStatus) {
      case 'success':
        // Build completed, deployment not started
        return stepName === 'deploying' ? 'wait' : 'wait';
      case 'deploying':
        return stepName === 'deploying' ? 'process' : 'wait';
      case 'deployed':
        return 'finish';
      case 'failed':
        return stepName === 'deploying' ? 'error' : (stepName === 'build-complete' ? 'finish' : 'wait');
      default:
        return 'wait';
    }
  };

  const formatDuration = (startTime, endTime) => {
    if (!startTime) return '-';
    const start = new Date(startTime);
    const end = endTime ? new Date(endTime) : new Date();
    const duration = Math.round((end - start) / 1000);
    return `${duration}s`;
  };

  const toggleAutoRefresh = () => {
    setAutoRefresh(!autoRefresh);
  };

  if (loading && !release) {
    return (
      <div style={{ padding: 24, textAlign: 'center' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!release) {
    return <div style={{ padding: 24 }}>Release not found</div>;
  }

  const isDeploying = release.status === 'deploying';
  const canDeploy = release.status === 'success';
  const canRollback = release.status === 'deployed';

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={
          <Space>
            <Link to={`/apps/${release.app_id}`}>
              <Button type="text" icon={<ArrowLeftOutlined />} />
            </Link>
            Deployment Details - Release #{release.id}
            {getStatusBadge(release.status, true)}
          </Space>
        }
        extra={
          <Space>
            <Button 
              size="small"
              type={autoRefresh ? "primary" : "default"}
              onClick={toggleAutoRefresh}
            >
              Auto Refresh {autoRefresh && isDeploying ? '(ON)' : '(OFF)'}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={() => fetchRelease()}>
              Refresh
            </Button>
            {canDeploy && (
              <Button 
                type="primary"
                icon={<PlayCircleOutlined />}
                onClick={showDeployModal}
              >
                Deploy
              </Button>
            )}
            {canRollback && (
              <Button 
                icon={<RollbackOutlined />}
                onClick={handleRollback}
                danger
              >
                Rollback
              </Button>
            )}
            <Link to={`/releases/${id}/build`}>
              <Button icon={<BuildOutlined />}>
                View Build
              </Button>
            </Link>
          </Space>
        }
      >
        {/* Release Information */}
        <Descriptions column={2} style={{ marginBottom: 24 }}>
          <Descriptions.Item label="Application">
            <Link to={`/apps/${release.app_id}`}>
              {release.app?.name || `App ${release.app_id}`}
            </Link>
          </Descriptions.Item>
          <Descriptions.Item label="Branch">
            <Tag color="blue">{release.branch}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Commit SHA">
            {release.commit_sha ? <code>{release.commit_sha}</code> : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Image Tag">
            {release.image_tag ? <code>{release.image_tag}</code> : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Started">
            {release.started_at ? new Date(release.started_at).toLocaleString() : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Duration">
            {formatDuration(release.started_at, release.finished_at)}
          </Descriptions.Item>
        </Descriptions>

        {/* Deployment Pipeline */}
        <Card title="Deployment Pipeline" size="small" style={{ marginBottom: 24 }}>
          <Steps direction="vertical" size="small">
            <Step
              title="Build Complete"
              description="Container image built and pushed to registry"
              status={getStepStatus('build-complete', release.status)}
            />
            <Step
              title="Deploy to Kubernetes"
              description="Update deployment with new image"
              status={getStepStatus('deploying', release.status)}
            />
            <Step
              title="Rollout Status"
              description="Wait for pods to be ready"
              status={getStepStatus('rollout', release.status)}
            />
            <Step
              title="Verify Deployment"
              description="Confirm deployment is healthy"
              status={getStepStatus('verify', release.status)}
            />
          </Steps>
        </Card>

        {/* Deployment Logs */}
        {logs.length > 0 && (
          <Card title="Deployment Logs" size="small" style={{ marginBottom: 24 }}>
            <div 
              style={{ 
                background: '#f5f5f5', 
                padding: 12, 
                borderRadius: 4,
                fontSize: '12px',
                maxHeight: 400,
                overflow: 'auto',
                fontFamily: 'monospace'
              }}
            >
              {logs.map((log, index) => (
                <div key={index} style={{ marginBottom: 4 }}>
                  <Text type="secondary">[{log.timestamp}]</Text>
                  <Text strong> {log.task}:</Text> {log.content}
                </div>
              ))}
            </div>
          </Card>
        )}

        {/* Status Alerts */}
        {release.status === 'success' && (
          <Alert
            type="info"
            message="Ready to Deploy"
            description="The build has completed successfully and is ready for deployment."
            showIcon
            style={{ marginTop: 16 }}
          />
        )}

        {release.status === 'deploying' && (
          <Alert
            type="info"
            message="Deployment in Progress"
            description="The deployment is currently running. This page will auto-refresh to show the latest status."
            showIcon
            style={{ marginTop: 16 }}
          />
        )}

        {release.status === 'deployed' && (
          <Alert
            type="success"
            message="Deployment Successful"
            description="The application has been successfully deployed to Kubernetes."
            showIcon
            action={
              <Button icon={<RollbackOutlined />} onClick={handleRollback} danger>
                Rollback
              </Button>
            }
            style={{ marginTop: 16 }}
          />
        )}

        {release.status === 'failed' && (
          <Alert
            type="error"
            message="Deployment Failed"
            description="The deployment process failed. Check the logs above for details."
            showIcon
            style={{ marginTop: 16 }}
          />
        )}
      </Card>

      {/* Deploy Configuration Modal */}
      <Modal
        title={
          <Space>
            <SettingOutlined />
            Deploy Configuration - Release #{release?.id}
          </Space>
        }
        visible={deployModalVisible}
        onOk={handleDeployModalOk}
        onCancel={() => setDeployModalVisible(false)}
        width={600}
        okText="Deploy"
        cancelText="Cancel"
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

export default DeployDetails;