import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router';
import { AuthProvider } from './stores/auth';
import ProtectedRoute from './components/ProtectedRoute';
import Layout from './components/Layout';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import AuditPage from './pages/AuditPage';
import UsersPage from './pages/UsersPage';
import TransfersPage from './pages/TransfersPage';
import ApiKeysPage from './pages/ApiKeysPage';
import StorageSettingsPage from './pages/StorageSettingsPage';
import StorageFilesPage from './pages/StorageFilesPage';
import StorageInsightsPage from './pages/StorageInsightsPage';
import LogRetentionPage from './pages/LogRetentionPage';
import SecuritySettingsPage from './pages/SecuritySettingsPage';
import BrandingPage from './pages/BrandingPage';
import GeneralSettingsPage from './pages/GeneralSettingsPage';
import SmtpSettingsPage from './pages/SmtpSettingsPage';
import EmailContentPage from './pages/EmailContentPage';
import AccountPage from './pages/AccountPage';

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />

          <Route element={<ProtectedRoute />}>
            <Route element={<Layout />}>
              <Route path="/" element={<DashboardPage />} />
              <Route path="/users" element={<UsersPage />} />
              <Route path="/transfers" element={<TransfersPage />} />
              <Route path="/apikeys" element={<ApiKeysPage />} />
              <Route path="/audit" element={<AuditPage />} />
              <Route path="/audit/settings" element={<LogRetentionPage />} />
              <Route path="/security" element={<SecuritySettingsPage />} />
              <Route path="/branding" element={<BrandingPage />} />
              <Route path="/general" element={<GeneralSettingsPage />} />
              <Route path="/storage" element={<StorageSettingsPage />} />
              <Route path="/storage/files" element={<StorageFilesPage />} />
              <Route path="/storage/insights" element={<StorageInsightsPage />} />
              <Route path="/email" element={<SmtpSettingsPage />} />
              <Route path="/email/content" element={<EmailContentPage />} />
              <Route path="/account" element={<AccountPage />} />
            </Route>
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
