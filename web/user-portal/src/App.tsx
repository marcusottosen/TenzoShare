import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router';
import { AuthProvider } from './stores/auth';
import ProtectedRoute from './components/ProtectedRoute';
import Layout from './components/Layout';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import MFAPage from './pages/MFAPage';
import ForgotPasswordPage from './pages/ForgotPasswordPage';
import DashboardPage from './pages/DashboardPage';
import FilesPage from './pages/FilesPage';
import NewTransferPage from './pages/NewTransferPage';
import TransferDetailPage from './pages/TransferDetailPage';
import SettingsPage from './pages/SettingsPage';
import ProfilePage from './pages/ProfilePage';
import RequestsPage from './pages/RequestsPage';
import FileRequestDetailPage from './pages/FileRequestDetailPage';
import SharesPage from './pages/SharesPage';

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/login/mfa" element={<MFAPage />} />
          <Route path="/forgot-password" element={<ForgotPasswordPage />} />

          {/* Protected routes */}
          <Route element={<ProtectedRoute />}>
            <Route element={<Layout />}>
              <Route path="/" element={<DashboardPage />} />
              <Route path="/files" element={<FilesPage />} />
              <Route path="/transfers/new" element={<NewTransferPage />} />
              <Route path="/transfers/:id" element={<TransferDetailPage />} />
              <Route path="/requests" element={<RequestsPage />} />
              <Route path="/requests/:id" element={<FileRequestDetailPage />} />
              <Route path="/shares" element={<SharesPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/profile" element={<ProfilePage />} />
            </Route>
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
