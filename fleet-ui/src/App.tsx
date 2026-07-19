import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider } from './auth/AuthContext'
import { ProtectedRoute } from './auth/ProtectedRoute'
import { ToastProvider } from './components/ToastContext'
import { AdminRoute } from './pages/AdminRoute'
import { DeviceDetailPage } from './pages/DeviceDetailPage'
import { DeviceWallPage } from './pages/DeviceWallPage'
import { LoginPage } from './pages/LoginPage'

const queryClient = new QueryClient()

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <ToastProvider>
          <BrowserRouter>
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route
                path="/"
                element={
                  <ProtectedRoute>
                    <DeviceWallPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/device/:udid"
                element={
                  <ProtectedRoute>
                    <DeviceDetailPage />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/admin/:tab?"
                element={
                  <ProtectedRoute>
                    <AdminRoute />
                  </ProtectedRoute>
                }
              />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </BrowserRouter>
        </ToastProvider>
      </AuthProvider>
    </QueryClientProvider>
  )
}
