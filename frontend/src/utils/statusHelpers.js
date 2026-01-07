import { Badge, Space } from 'antd';
import { 
  ClockCircleOutlined,
  PlayCircleOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined
} from '@ant-design/icons';

/**
 * Status configuration mapping for different states
 */
const statusConfig = {
  pending: { status: 'default', text: 'Pending', icon: <ClockCircleOutlined /> },
  running: { status: 'processing', text: 'Running', icon: <PlayCircleOutlined /> },
  deploying: { status: 'processing', text: 'Deploying', icon: <PlayCircleOutlined /> },
  success: { status: 'success', text: 'Success', icon: <CheckCircleOutlined /> },
  deployed: { status: 'success', text: 'Deployed', icon: <CheckCircleOutlined /> },
  built: { status: 'success', text: 'Built', icon: <CheckCircleOutlined /> },
  failed: { status: 'error', text: 'Failed', icon: <ExclamationCircleOutlined /> },
  canceled: { status: 'default', text: 'Canceled', icon: <ExclamationCircleOutlined /> },
  cancelled: { status: 'default', text: 'Canceled', icon: <ExclamationCircleOutlined /> },
  error: { status: 'error', text: 'Error', icon: <ExclamationCircleOutlined /> },
  completed: { status: 'success', text: 'Completed', icon: <CheckCircleOutlined /> },
  stopped: { status: 'default', text: 'Stopped', icon: <ExclamationCircleOutlined /> },
  unknown: { status: 'default', text: 'Unknown', icon: null }
};

/**
 * Generate a status badge component based on the status string
 * @param {string} status - The status string
 * @param {boolean} showIcon - Whether to show icons with text (default: false)
 * @returns {JSX.Element} Badge component with appropriate styling
 */
export const getStatusBadge = (status, showIcon = false) => {
  const config = statusConfig[status?.toLowerCase()] || { status: 'default', text: status || 'Unknown', icon: null };
  
  if (showIcon && config.icon) {
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
  }
  
  return <Badge status={config.status} text={config.text} />;
};

/**
 * Get status configuration object (useful for styling without JSX)
 * @param {string} status - The status string
 * @returns {Object} Status configuration with status and text properties
 */
export const getStatusConfig = (status) => {
  return statusConfig[status?.toLowerCase()] || { status: 'default', text: status || 'Unknown' };
};

/**
 * Check if a status represents a terminal/final state
 * @param {string} status - The status string
 * @returns {boolean} True if status is terminal (success, failed, canceled, etc.)
 */
export const isTerminalStatus = (status) => {
  const terminalStates = ['success', 'failed', 'canceled', 'cancelled', 'error', 'completed', 'stopped'];
  return terminalStates.includes(status?.toLowerCase());
};

/**
 * Check if a status represents an active/running state
 * @param {string} status - The status string
 * @returns {boolean} True if status is active (running, pending)
 */
export const isActiveStatus = (status) => {
  const activeStates = ['running', 'pending'];
  return activeStates.includes(status?.toLowerCase());
};