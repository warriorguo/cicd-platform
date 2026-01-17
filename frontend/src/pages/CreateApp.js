import React, { useState } from 'react';
import { Form, Input, InputNumber, Button, Card, message, Space, Select, Collapse } from 'antd';
import { ArrowLeftOutlined, SettingOutlined } from '@ant-design/icons';
import { Link, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';

const { Option } = Select;

const CreateApp = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [buildType, setBuildType] = useState('dockerfile');
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
            build_type: 'dockerfile',
            dockerfile_path: 'Dockerfile',
            service_port: 8080,
          }}
          onValuesChange={(changedValues) => {
            if (changedValues.build_type) {
              setBuildType(changedValues.build_type);
              // Update dockerfile_path based on build_type
              if (changedValues.build_type === 'docker-compose') {
                form.setFieldsValue({ dockerfile_path: 'docker-compose.yml' });
              } else {
                form.setFieldsValue({ dockerfile_path: 'Dockerfile' });
              }
            }
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
            label="Build Type"
            name="build_type"
            tooltip="Select build method: Dockerfile or docker-compose"
          >
            <Select>
              <Option value="dockerfile">Dockerfile</Option>
              <Option value="docker-compose">Docker Compose</Option>
            </Select>
          </Form.Item>

          <Form.Item
            label={buildType === 'docker-compose' ? 'Docker Compose File Path' : 'Dockerfile Path'}
            name="dockerfile_path"
            tooltip={buildType === 'docker-compose'
              ? 'Path to docker-compose.yml relative to repository root'
              : 'Path to Dockerfile relative to repository root'}
          >
            <Input placeholder={buildType === 'docker-compose' ? 'docker-compose.yml' : 'Dockerfile'} />
          </Form.Item>

          <Form.Item
            label="Service Port"
            name="service_port"
            tooltip="The port your application container will expose and listen on"
            rules={[
              { required: true, message: 'Please enter service port' },
              { type: 'number', min: 1, max: 65535, message: 'Port must be between 1 and 65535' },
            ]}
          >
            <InputNumber
              placeholder="8080"
              style={{ width: '100%' }}
              min={1}
              max={65535}
            />
          </Form.Item>

          <Collapse
            ghost
            style={{ marginBottom: 24 }}
            items={[
              {
                key: 'advanced',
                label: (
                  <Space>
                    <SettingOutlined />
                    Advanced Options
                  </Space>
                ),
                children: (
                  <>
                    <Form.Item
                      label="Service Account"
                      name="service_account"
                      tooltip="Kubernetes ServiceAccount for the deployment. Leave empty to use default. Required for apps that need to interact with Kubernetes API."
                    >
                      <Input placeholder="e.g., cicd-platform (leave empty for default)" />
                    </Form.Item>

                    <Form.Item
                      label="Git Secret Reference"
                      name="git_secret_ref"
                      tooltip="Name of Kubernetes secret containing Git credentials for private repositories"
                    >
                      <Input placeholder="e.g., git-credentials" />
                    </Form.Item>

                    <Form.Item
                      label="Registry Secret Reference"
                      name="registry_secret_ref"
                      tooltip="Name of Kubernetes secret containing registry credentials for pushing images"
                    >
                      <Input placeholder="e.g., registry-credentials" />
                    </Form.Item>
                  </>
                ),
              },
            ]}
          />

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