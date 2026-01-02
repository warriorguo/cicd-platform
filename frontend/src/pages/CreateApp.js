import React, { useState } from 'react';
import { Form, Input, Button, Card, message, Space } from 'antd';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { Link, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';

const CreateApp = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const onFinish = async (values) => {
    setLoading(true);
    try {
      const response = await appsAPI.createApp(values);
      message.success('Application created successfully!');
      navigate(`/apps/${response.data.id}`);
    } catch (error) {
      message.error('Failed to create application');
      console.error('Error creating app:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={
          <Space>
            <Link to="/apps">
              <Button type="text" icon={<ArrowLeftOutlined />} />
            </Link>
            Create Application
          </Space>
        }
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{
            default_branch: 'main',
            dockerfile_path: 'Dockerfile',
            context_path: '.',
          }}
        >
          <Form.Item
            label="Application Name"
            name="name"
            rules={[
              { required: true, message: 'Please enter application name' },
              { pattern: /^[a-z0-9-]+$/, message: 'Only lowercase letters, numbers, and hyphens allowed' },
            ]}
          >
            <Input placeholder="my-app" />
          </Form.Item>

          <Form.Item
            label="Git Repository URL"
            name="git_url"
            rules={[
              { required: true, message: 'Please enter Git repository URL' },
              { type: 'url', message: 'Please enter a valid URL' },
            ]}
          >
            <Input placeholder="https://github.com/username/repo.git" />
          </Form.Item>

          <Form.Item
            label="Default Branch"
            name="default_branch"
          >
            <Input placeholder="main" />
          </Form.Item>

          <Form.Item
            label="Dockerfile Path"
            name="dockerfile_path"
            tooltip="Path to Dockerfile relative to repository root"
          >
            <Input placeholder="Dockerfile" />
          </Form.Item>

          <Form.Item
            label="Build Context Path"
            name="context_path"
            tooltip="Build context directory relative to repository root"
          >
            <Input placeholder="." />
          </Form.Item>

          <Form.Item
            label="Container Registry Repository"
            name="registry_repo"
            rules={[{ required: true, message: 'Please enter registry repository' }]}
          >
            <Input placeholder="harbor.company.com/team/my-app" />
          </Form.Item>

          <Form.Item
            label="Target Kubernetes Namespace"
            name="target_namespace"
            rules={[{ required: true, message: 'Please enter target namespace' }]}
          >
            <Input placeholder="production" />
          </Form.Item>

          <Form.Item
            label="Target Deployment Name"
            name="target_deploy_name"
            rules={[{ required: true, message: 'Please enter deployment name' }]}
          >
            <Input placeholder="my-app" />
          </Form.Item>

          <Form.Item
            label="Git Secret Reference (Optional)"
            name="git_secret_ref"
            tooltip="Kubernetes secret containing Git credentials"
          >
            <Input placeholder="git-credentials" />
          </Form.Item>

          <Form.Item
            label="Registry Secret Reference (Optional)"
            name="registry_secret_ref"
            tooltip="Kubernetes secret containing registry credentials"
          >
            <Input placeholder="registry-credentials" />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={loading}>
                Create Application
              </Button>
              <Link to="/apps">
                <Button>Cancel</Button>
              </Link>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default CreateApp;