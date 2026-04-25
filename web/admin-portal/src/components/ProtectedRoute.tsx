import React from 'react';
import { Navigate, Outlet } from 'react-router';
import { useAuth } from '../stores/auth';

export default function ProtectedRoute() {
  const { isAuthenticated, isLoading, user } = useAuth();
  if (isLoading) return <div className="page-center">Loading…</div>;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (user && user.role !== 'admin') {
    return (
      <div className="page-center">
        <div>Access denied. Admin role required.</div>
      </div>
    );
  }
  return <Outlet />;
}
