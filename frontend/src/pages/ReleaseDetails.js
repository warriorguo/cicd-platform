import React, { useState, useEffect } from 'react';
import { 
  Card, 
  Button, 
  Steps, 
  Alert, 
  Typography, 
  Space, 
  Badge,
  Tag,
  Descriptions,
  message 
} from 'antd';
import { 
  ArrowLeftOutlined, 
  ReloadOutlined,
  RollbackOutlined 
} from '@ant-design/icons';
import { Link, useParams } from 'react-router-dom';
import { appsAPI, createWebSocketConnection } from '../services/api';

const { Step } = Steps;
const { Text, Paragraph } = Typography;

const ReleaseDetails = () => {
  const { id } = useParams();
  const [release, setRelease] = useState(null);
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [wsConnection, setWsConnection] = useState(null);

  useEffect(() => {
    if (id) {
      fetchRelease();
      setupWebSocket();
    }

    return () => {
      if (wsConnection) {
        wsConnection.close();
      }
    };
  }, [id]);

  const fetchRelease = async () => {
    setLoading(true);
    try {
      const response = await appsAPI.getRelease(id);
      setRelease(response.data);
    } catch (error) {
      message.error('Failed to fetch release');
    } finally {
      setLoading(false);
    }
  };

  const setupWebSocket = () => {
    try {
      const ws = createWebSocketConnection(id);
      
      ws.onopen = () => {
        console.log('WebSocket connected');
      };

      ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        
        if (data.type === 'status') {
          setRelease(prev => ({
            ...prev,
            status: data.payload.status,
            started_at: data.payload.started_at,
            finished_at: data.payload.finished_at,
            image_digest: data.payload.image_digest,
          }));
        } else if (data.type === 'log') {
          setLogs(prev => [...prev, data.payload]);
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      ws.onclose = () => {
        console.log('WebSocket disconnected');
      };

      setWsConnection(ws);
    } catch (error) {
      console.error('Failed to setup WebSocket:', error);
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
    if (!release || release.status === 'pending') return 'wait';
    
    const stepOrder = ['git-clone', 'validate-dockerfile', 'kaniko-build', 'kubectl-deploy', 'verify-deployment'];
    const currentStepIndex = logs.reduce((maxIndex, log) => {
      const index = stepOrder.indexOf(log.task);
      return index > maxIndex ? index : maxIndex;
    }, -1);
    
    const stepIndex = stepOrder.indexOf(stepName);
    
    if (stepIndex <= currentStepIndex) {
      return releaseStatus === 'failed' && stepIndex === currentStepIndex ? 'error' : 'finish';
    } else if (stepIndex === currentStepIndex + 1 && releaseStatus === 'running') {
      return 'process';
    }
    
    return 'wait';
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

  if (loading || !release) {
    return <div style={{ padding: 24 }}>Loading...</div>;
  }

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={
          <Space>
            <Link to={`/apps/${release.app_id}`}>
              <Button type="text" icon={<ArrowLeftOutlined />} />
            </Link>
            Release #{release.id}
            {getStatusBadge(release.status)}
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={fetchRelease}>
              Refresh
            </Button>
            {release.status === 'success' && (
              <Button 
                icon={<RollbackOutlined />} 
                onClick={handleRollback}
                type="primary"
                danger
              >
                Rollback
              </Button>
            )}
          </Space>
        }
      >
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
          <Descriptions.Item label="Image Digest">
            {release.image_digest ? <code>{release.image_digest.substring(0, 20)}...</code> : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Started">
            {release.started_at ? new Date(release.started_at).toLocaleString() : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Finished">
            {release.finished_at ? new Date(release.finished_at).toLocaleString() : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Duration">
            {release.started_at && release.finished_at 
              ? `${Math.round((new Date(release.finished_at) - new Date(release.started_at)) / 1000)}s`
              : '-'
            }
          </Descriptions.Item>
        </Descriptions>

        {release.status !== 'pending' && (
          <Card title="Pipeline Progress" size="small" style={{ marginBottom: 24 }}>
            <Steps direction="vertical" size="small">
              <Step
                title="Git Clone"
                description="Clone source code from repository"
                status={getStepStatus('git-clone', release.status)}
              />
              <Step
                title="Validate Dockerfile"
                description="Verify Dockerfile exists and is valid"
                status={getStepStatus('validate-dockerfile', release.status)}
              />
              <Step
                title="Build & Push"
                description="Build container image and push to registry"
                status={getStepStatus('kaniko-build', release.status)}
              />
              <Step
                title="Deploy"
                description="Update Kubernetes deployment"
                status={getStepStatus('kubectl-deploy', release.status)}
              />
              <Step
                title="Verify"
                description="Verify deployment rollout"
                status={getStepStatus('verify-deployment', release.status)}
              />
            </Steps>
          </Card>
        )}

        {logs.length > 0 && (
          <Card title="Pipeline Logs" size="small">
            <div className="pipeline-logs">
              {logs.map((log, index) => (
                <div key={index} className="pipeline-stage">
                  <Text type="secondary">[{log.timestamp}]</Text>
                  <Text strong> {log.task}:</Text> {log.content}
                </div>
              ))}
            </div>
          </Card>
        )}

        {release.status === 'failed' && (
          <Alert
            type="error"
            message="Release Failed"
            description="The release pipeline failed during execution. Check the logs above for details."
            showIcon
            style={{ marginTop: 16 }}
          />
        )}

        {release.status === 'success' && (
          <Alert
            type="success"
            message="Release Successful"
            description="The application has been successfully deployed."
            showIcon
            style={{ marginTop: 16 }}
          />
        )}
      </Card>
    </div>
  );
};

export default ReleaseDetails;