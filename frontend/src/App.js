import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Layout, Button } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { Link, useLocation } from 'react-router-dom';

import AppsLayout from './pages/AppsLayout';
import CreateApp from './pages/CreateApp';
import BuildDetails from './pages/BuildDetails';
import DeployDetails from './pages/DeployDetails';

const { Header, Content } = Layout;

function App() {
  const location = useLocation();

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ padding: '0 24px', background: '#001529', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ color: 'white', fontSize: '18px', fontWeight: 'bold' }}>
          CI/CD Platform
        </div>
        {(location.pathname === '/apps' || location.pathname.startsWith('/apps/')) && !location.pathname.includes('/new') && (
          <Link to="/apps/new">
            <Button type="primary" icon={<PlusOutlined />}>
              Create App
            </Button>
          </Link>
        )}
      </Header>
      <Layout>
        <Content style={{ background: '#f0f2f5', minHeight: 280 }}>
          <Routes>
            <Route path="/" element={<Navigate to="/apps" replace />} />
            <Route path="/apps" element={<AppsLayout />} />
            <Route path="/apps/new" element={<CreateApp />} />
            <Route path="/apps/:id" element={<AppsLayout />} />
            <Route path="/releases/:id/build" element={<BuildDetails />} />
            <Route path="/releases/:id/deploy" element={<DeployDetails />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  );
}

export default App;