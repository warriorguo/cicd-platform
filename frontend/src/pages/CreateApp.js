import React, { useState } from 'react';
import { Form, Input, Button, Card, message, Space, Select, Divider, InputNumber } from 'antd';
import { ArrowLeftOutlined, PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
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
            context_path: '.',
            replicas: 1,
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
            label="Replicas"
            name="replicas"
            tooltip="Number of pod replicas to deploy"
            rules={[
              { required: true, message: 'Please enter number of replicas' },
              { type: 'number', min: 1, max: 100, message: 'Replicas must be between 1 and 100' }
            ]}
          >
            <InputNumber min={1} max={100} style={{ width: '100%' }} placeholder="1" />
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

          <Divider orientation="left">Environment Variables (Optional)</Divider>

          <Form.List name="env_vars">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Space
                    key={key}
                    style={{ display: 'flex', marginBottom: 8 }}
                    align="baseline"
                  >
                    <Form.Item
                      {...restField}
                      name={[name, 'name']}
                      rules={[
                        { required: true, message: 'Variable name required' },
                        { pattern: /^[A-Z_][A-Z0-9_]*$/, message: 'Use uppercase letters, numbers, and underscores' }
                      ]}
                    >
                      <Input placeholder="VARIABLE_NAME" style={{ width: 200 }} />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, 'value']}
                      rules={[{ required: true, message: 'Value required' }]}
                    >
                      <Input placeholder="value" style={{ width: 300 }} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(name)} />
                  </Space>
                ))}
                <Form.Item>
                  <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                    Add Environment Variable
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>

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