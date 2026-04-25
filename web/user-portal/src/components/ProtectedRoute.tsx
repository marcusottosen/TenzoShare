import React from 'react';
import { Navigate, Outlet } from 'react-router';
import { useAuth } from '../stores/auth';

export default function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth();
  if (isLoading) return <div className="page-center">Loading…</div>;
  return isAuthenticated ? <Outlet /> : <Navigate to="/login" replace />;
}
