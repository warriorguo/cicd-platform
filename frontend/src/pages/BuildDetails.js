import React, { useState, useEffect, useRef } from 'react';
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
  message,
  Progress,
  Collapse,
  Spin,
  Row,
  Col
} from 'antd';
import { 
  ArrowLeftOutlined, 
  ReloadOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  ClockCircleOutlined,
  EyeOutlined
} from '@ant-design/icons';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';

const { Step } = Steps;
const { Text, Paragraph } = Typography;
const { Panel } = Collapse;

const BuildDetails = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [release, setRelease] = useState(null);
  const [buildStatus, setBuildStatus] = useState(null);
  const [buildStages, setBuildStages] = useState([]);
  const [buildLogs, setBuildLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const intervalRef = useRef(null);

  useEffect(() => {
    if (id) {
      fetchAllData();
      setupAutoRefresh();
    }

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [id]);

  const setupAutoRefresh = () => {
    // Set up auto refresh every 5 seconds when build is running
    intervalRef.current = setInterval(() => {
      if (autoRefresh && buildStatus && 
          (buildStatus.overall_status === 'running' || buildStatus.overall_status === 'pending')) {
        fetchAllData(false); // Don't show loading spinner for auto refresh
      }
    }, 5000);
  };

  const fetchAllData = async (showLoading = true) => {
    if (showLoading) setLoading(true);
    try {
      // Fetch release basic info
      const releaseResponse = await appsAPI.getRelease(id);
      setRelease(releaseResponse.data);

      // Fetch build status
      const statusResponse = await appsAPI.getBuildStatus(id);
      setBuildStatus(statusResponse.data);

      // Fetch build stages
      const stagesResponse = await appsAPI.getBuildStages(id);
      setBuildStages(stagesResponse.data.stages || []);

      // Fetch build logs
      const logsResponse = await appsAPI.getBuildLogs(id);
      setBuildLogs(logsResponse.data.logs || []);
      
      // Stop auto refresh if build is completed
      if (statusResponse.data.overall_status === 'success' || 
          statusResponse.data.overall_status === 'failed') {
        setAutoRefresh(false);
      }
    } catch (error) {
      message.error('Failed to fetch build data');
    } finally {
      if (showLoading) setLoading(false);
    }
  };

  const getStatusBadge = (status) => {
    const statusConfig = {
      pending: { status: 'default', text: 'Pending', icon: <ClockCircleOutlined /> },
      running: { status: 'processing', text: 'Running', icon: <PlayCircleOutlined /> },
      completed: { status: 'success', text: 'Completed', icon: <CheckCircleOutlined /> },
      success: { status: 'success', text: 'Success', icon: <CheckCircleOutlined /> },
      failed: { status: 'error', text: 'Failed', icon: <ExclamationCircleOutlined /> },
      canceled: { status: 'default', text: 'Canceled', icon: <ExclamationCircleOutlined /> },
    };
    const config = statusConfig[status] || { status: 'default', text: status, icon: null };
    return (
      <Badge 
        status={config.status} 
        text={
          <Space>
            {config.icon}
            {config.text}
          </Space>
        } 
      />
    );
  };

  const getStepStatus = (stageStatus) => {
    switch (stageStatus) {
      case 'completed': return 'finish';
      case 'running': return 'process';
      case 'failed': return 'error';
      default: return 'wait';
    }
  };

  const handleDeploy = async () => {
    try {
      await appsAPI.deployRelease(id);
      message.success('Deployment initiated');
      navigate(`/releases/${id}/deploy`);
    } catch (error) {
      message.error('Failed to start deployment');
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

  const isRunning = buildStatus?.overall_status === 'running' || buildStatus?.overall_status === 'pending';
  const canDeploy = buildStatus?.overall_status === 'success';

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={
          <Space>
            <Link to={`/apps/${release.app_id}`}>
              <Button type="text" icon={<ArrowLeftOutlined />} />
            </Link>
            Build Details - Release #{release.id}
            {buildStatus && getStatusBadge(buildStatus.overall_status)}
          </Space>
        }
        extra={
          <Space>
            <Button 
              size="small"
              type={autoRefresh ? "primary" : "default"}
              onClick={toggleAutoRefresh}
            >
              Auto Refresh {autoRefresh && isRunning ? '(ON)' : '(OFF)'}
            </Button>
            <Button icon={<ReloadOutlined />} onClick={() => fetchAllData()}>
              Refresh
            </Button>
            {canDeploy && (
              <Button 
                type="primary"
                icon={<PlayCircleOutlined />}
                onClick={handleDeploy}
              >
                Deploy
              </Button>
            )}
            <Link to={`/releases/${id}`}>
              <Button icon={<EyeOutlined />}>
                View Legacy
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
            {buildStatus?.started_at ? new Date(buildStatus.started_at).toLocaleString() : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Duration">
            {formatDuration(buildStatus?.started_at, buildStatus?.finished_at)}
          </Descriptions.Item>
        </Descriptions>

        {/* Build Progress */}
        {buildStatus && (
          <Card title="Build Progress" size="small" style={{ marginBottom: 24 }}>
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <div>
                  <Text strong>Overall Progress</Text>
                  <Progress 
                    percent={buildStatus.progress_percentage} 
                    status={buildStatus.overall_status === 'failed' ? 'exception' : 'active'}
                    strokeColor={buildStatus.overall_status === 'success' ? '#52c41a' : undefined}
                  />
                </div>
              </Col>
              <Col span={12}>
                <div>
                  <Text strong>Stages: </Text>
                  <Text>{buildStatus.completed_stages} / {buildStatus.total_stages} completed</Text>
                  {buildStatus.current_stage && (
                    <div>
                      <Text strong>Current: </Text>
                      <Tag color="processing">{buildStatus.current_stage}</Tag>
                    </div>
                  )}
                </div>
              </Col>
            </Row>
          </Card>
        )}

        {/* Build Stages */}
        {buildStages && buildStages.length > 0 && (
          <Card title="Build Stages" size="small" style={{ marginBottom: 24 }}>
            <Steps direction="vertical" size="small">
              {buildStages.map((stage, index) => (
                <Step
                  key={stage.stage_name}
                  title={stage.stage_name}
                  description={
                    <Space direction="vertical" size="small">
                      <Text type="secondary">
                        {stage.containers} container{stage.containers > 1 ? 's' : ''}
                        {stage.started_at && ` • Started: ${new Date(stage.started_at).toLocaleTimeString()}`}
                        {stage.finished_at && ` • Finished: ${new Date(stage.finished_at).toLocaleTimeString()}`}
                      </Text>
                    </Space>
                  }
                  status={getStepStatus(stage.status)}
                />
              ))}
            </Steps>
          </Card>
        )}

        {/* Build Logs */}
        {buildLogs && buildLogs.length > 0 && (
          <Card title="Build Logs" size="small">
            <Collapse>
              {buildStages.map((stage) => {
                const stageLogs = buildLogs.filter(log => log.stage_name === stage.stage_name);
                return stageLogs.length > 0 ? (
                  <Panel 
                    header={
                      <Space>
                        <span>{stage.stage_name}</span>
                        {getStatusBadge(stage.status)}
                        <Text type="secondary">({stageLogs.length} containers)</Text>
                      </Space>
                    } 
                    key={stage.stage_name}
                  >
                    {stageLogs.map((log, index) => (
                      <Card 
                        key={log.id} 
                        size="small" 
                        title={
                          <Space>
                            <code>{log.container_name}</code>
                            {getStatusBadge(log.status)}
                          </Space>
                        }
                        style={{ marginBottom: 8 }}
                      >
                        {log.logs_content ? (
                          <pre 
                            style={{ 
                              background: '#f5f5f5', 
                              padding: 12, 
                              borderRadius: 4,
                              fontSize: '12px',
                              maxHeight: 300,
                              overflow: 'auto'
                            }}
                          >
                            {log.logs_content}
                          </pre>
                        ) : (
                          <Text type="secondary">No logs available yet</Text>
                        )}
                        {log.error_message && (
                          <Alert
                            message="Error"
                            description={log.error_message}
                            type="error"
                            size="small"
                            style={{ marginTop: 8 }}
                          />
                        )}
                      </Card>
                    ))}
                  </Panel>
                ) : null;
              })}
            </Collapse>
          </Card>
        )}

        {/* Status Alerts */}
        {buildStatus?.overall_status === 'failed' && (
          <Alert
            type="error"
            message="Build Failed"
            description="The build pipeline failed during execution. Check the logs above for details."
            showIcon
            style={{ marginTop: 16 }}
          />
        )}

        {buildStatus?.overall_status === 'success' && (
          <Alert
            type="success"
            message="Build Successful"
            description="The container image has been successfully built and pushed to the registry. You can now deploy this release."
            showIcon
            action={
              <Button type="primary" onClick={handleDeploy}>
                Deploy Now
              </Button>
            }
            style={{ marginTop: 16 }}
          />
        )}
      </Card>
    </div>
  );
};

export default BuildDetails;