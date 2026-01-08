import React, { useState, useEffect } from 'react';
import { Table, Card, message, Row, Col, Empty, Button } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';
import AppDetails from './AppDetails';

const AppsLayout = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const [apps, setApps] = useState([]);
  const [loading, setLoading] = useState(false);
  const [selectedAppId, setSelectedAppId] = useState(id);

  useEffect(() => {
    fetchApps();
  }, []);

  useEffect(() => {
    setSelectedAppId(id);
  }, [id]);

  const fetchApps = async () => {
    setLoading(true);
    try {
      const response = await appsAPI.getApps();
      // Sort by created_at in descending order (newest first)
      const sortedApps = response.data.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
      setApps(sortedApps);
    } catch (error) {
      message.error('Failed to fetch applications');
      console.error('Error fetching apps:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleAppSelect = (app) => {
    navigate(`/apps/${app.id}`);
  };

  const columns = [
    {
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => (
        <div 
          onClick={() => handleAppSelect(record)}
          style={{ 
            fontWeight: 'bold',
            cursor: 'pointer',
            color: selectedAppId === record.id.toString() ? '#1890ff' : 'inherit'
          }}
        >
          {text}
        </div>
      ),
    },
  ];

  return (
    <div style={{ padding: 24, height: 'calc(100vh - 64px)', overflow: 'hidden' }}>
      <Row gutter={24} style={{ height: '100%' }}>
        {/* Left panel - Applications list */}
        <Col span={5} style={{ height: '100%', overflow: 'auto' }}>
          <Card
            title="Applications"
            style={{ height: '100%' }}
            bodyStyle={{ padding: '12px' }}
            extra={
              <Button icon={<ReloadOutlined />} onClick={fetchApps} size="small">
                Refresh
              </Button>
            }
          >
            <Table
              columns={columns}
              dataSource={apps}
              loading={loading}
              rowKey="id"
              pagination={{
                pageSize: 15,
                showSizeChanger: false,
                size: 'small'
              }}
              size="small"
              showHeader={false}
              onRow={(record) => ({
                onClick: () => handleAppSelect(record),
                style: {
                  cursor: 'pointer',
                  backgroundColor: selectedAppId === record.id.toString() ? '#e6f7ff' : 'inherit',
                  padding: '4px 8px'
                }
              })}
            />
          </Card>
        </Col>

        {/* Right panel - Application details */}
        <Col span={19} style={{ height: '100%', overflow: 'auto' }}>
          {selectedAppId ? (
            <div style={{ height: '100%' }}>
              <AppDetails onAppDeleted={fetchApps} />
            </div>
          ) : (
            <Card style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Empty 
                description="Select an application from the list to view details"
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            </Card>
          )}
        </Col>
      </Row>
    </div>
  );
};

export default AppsLayout;