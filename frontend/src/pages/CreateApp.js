import React, { useState } from 'react';
import { Form, Input, InputNumber, Button, Card, message, Space, Select, Collapse, Radio } from 'antd';
import { ArrowLeftOutlined, SettingOutlined } from '@ant-design/icons';
import { Link, useNavigate } from 'react-router-dom';
import { appsAPI } from '../services/api';

const { Option } = Select;

const CreateApp = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [buildType, setBuildType] = useState('dockerfile');
  const [buildMode, setBuildMode] = useState('source'); // 'source' or 'external'
  const navigate = useNavigate();

  const onFinish = async (values) => {
    setLoading(true);
    try {
      const payload = { name: values.name, service_port: values.service_port };

      if (buildMode === 'external') {
        payload.build_type = 'external-image';
        payload.external_image = values.external_image;
      } else {
        payload.git_url = values.git_url;
        payload.default_branch = values.default_branch;
        payload.build_type = values.build_type;
        payload.dockerfile_path = values.dockerfile_path;
        if (values.service_account) payload.service_account = values.service_account;
        if (values.git_secret_ref) payload.git_secret_ref = values.git_secret_ref;
        if (values.registry_secret_ref) payload.registry_secret_ref = values.registry_secret_ref;
      }

      const response = await appsAPI.createApp(payload);
      message.success('Application created successfully!');
      navigate(`/apps/${response.data.id}`);
    } catch (error) {
      message.error(error.response?.data?.error || 'Failed to create application');
      console.error('Error creating app:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleBuildModeChange = (e) => {
    const mode = e.target.value;
    setBuildMode(mode);
    if (mode === 'external') {
      form.setFieldsValue({ build_type: 'external-image' });
    } else {
      form.setFieldsValue({ build_type: 'dockerfile' });
      setBuildType('dockerfile');
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
            if (changedValues.build_type && buildMode === 'source') {
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

          <Form.Item label="Build Mode">
            <Radio.Group value={buildMode} onChange={handleBuildModeChange}>
              <Radio.Button value="source">Build from Source</Radio.Button>
              <Radio.Button value="external">External Image</Radio.Button>
            </Radio.Group>
          </Form.Item>

          {buildMode === 'external' ? (
            <Form.Item
              label="Image Address"
              name="external_image"
              rules={[
                { required: true, message: 'Please enter image address' },
              ]}
              tooltip="Full image address including registry and tag"
            >
              <Input placeholder="nginx:latest or gcr.io/project/app:v1" />
            </Form.Item>
          ) : (
            <>
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
            </>
          )}

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

          {buildMode === 'source' && (
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
          )}

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
