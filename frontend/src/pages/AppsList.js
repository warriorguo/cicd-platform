import React, { useState, useEffect } from 'react';
import { Button, Table, Card, message, Space, Tag } from 'antd';
import { PlusOutlined, EyeOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import { appsAPI } from '../services/api';

const AppsList = () => {
  const [apps, setApps] = useState([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchApps();
  }, []);

  const fetchApps = async () => {
    setLoading(true);
    try {
      const response = await appsAPI.getApps();
      setApps(response.data);
    } catch (error) {
      message.error('Failed to fetch applications');
      console.error('Error fetching apps:', error);
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => (
        <Link to={`/apps/${record.id}`} style={{ fontWeight: 'bold' }}>
          {text}
        </Link>
      ),
    },
    {
      title: 'Git Repository',
      dataIndex: 'git_url',
      key: 'git_url',
      render: (text) => (
        <a href={text} target="_blank" rel="noopener noreferrer">
          {text}
        </a>
      ),
    },
    {
      title: 'Default Branch',
      dataIndex: 'default_branch',
      key: 'default_branch',
      render: (text) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: 'Registry',
      dataIndex: 'registry_repo',
      key: 'registry_repo',
    },
    {
      title: 'Target Namespace',
      dataIndex: 'target_namespace',
      key: 'target_namespace',
      render: (text) => <Tag color="green">{text}</Tag>,
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text) => new Date(text).toLocaleDateString(),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <Space size="middle">
          <Link to={`/apps/${record.id}`}>
            <Button type="link" icon={<EyeOutlined />} size="small">
              View
            </Button>
          </Link>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="Applications"
        extra={
          <Link to="/apps/new">
            <Button type="primary" icon={<PlusOutlined />}>
              Create App
            </Button>
          </Link>
        }
      >
        <Table
          columns={columns}
          dataSource={apps}
          loading={loading}
          rowKey="id"
          pagination={{
            pageSize: 10,
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Card>
    </div>
  );
};

export default AppsList;