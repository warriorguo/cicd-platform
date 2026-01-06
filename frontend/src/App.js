import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Layout, Menu } from 'antd';
import { AppstoreOutlined, DeploymentUnitOutlined } from '@ant-design/icons';
import { Link, useLocation } from 'react-router-dom';

import AppsList from './pages/AppsList';
import AppDetails from './pages/AppDetails';
import CreateApp from './pages/CreateApp';
import ReleaseDetails from './pages/ReleaseDetails';
import BuildDetails from './pages/BuildDetails';
import DeployDetails from './pages/DeployDetails';

const { Header, Content, Sider } = Layout;

function App() {
  const location = useLocation();

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ padding: '0 24px', background: '#001529' }}>
        <div style={{ color: 'white', fontSize: '18px', fontWeight: 'bold' }}>
          CI/CD Platform
        </div>
      </Header>
      <Layout>
        <Sider width={250} style={{ background: '#fff' }}>
          <Menu
            mode="inline"
            selectedKeys={[location.pathname]}
            style={{ height: '100%', borderRight: 0 }}
          >
            <Menu.Item key="/apps" icon={<AppstoreOutlined />}>
              <Link to="/apps">Applications</Link>
            </Menu.Item>
          </Menu>
        </Sider>
        <Layout style={{ padding: '0' }}>
          <Content style={{ background: '#f0f2f5', minHeight: 280 }}>
            <Routes>
              <Route path="/" element={<Navigate to="/apps" replace />} />
              <Route path="/apps" element={<AppsList />} />
              <Route path="/apps/new" element={<CreateApp />} />
              <Route path="/apps/:id" element={<AppDetails />} />
              <Route path="/releases/:id" element={<ReleaseDetails />} />
              <Route path="/releases/:id/build" element={<BuildDetails />} />
              <Route path="/releases/:id/deploy" element={<DeployDetails />} />
            </Routes>
          </Content>
        </Layout>
      </Layout>
    </Layout>
  );
}

export default App;