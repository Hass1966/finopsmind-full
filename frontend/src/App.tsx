import { useState } from 'react'
import { BrowserRouter, Routes, Route, Link, useLocation, Navigate } from 'react-router-dom'
import {
  LayoutDashboard, DollarSign, AlertTriangle, TrendingUp, Lightbulb, Settings, Menu,
  Wrench, Shield, MessageSquare, Container, Calculator, Wallet, GitCompare
} from 'lucide-react'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { LoadingSpinner } from './components/shared'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import CostsPage from './pages/CostsPage'
import AnomaliesPage from './pages/AnomaliesPage'
import ForecastsPage from './pages/ForecastsPage'
import RecommendationsPage from './pages/RecommendationsPage'
import SettingsPage from './pages/SettingsPage'
import RemediationPage from './pages/RemediationPage'
import PoliciesPage from './pages/PoliciesPage'
import ChatPage from './pages/ChatPage'
import KubernetesPage from './pages/KubernetesPage'
import UnitEconomicsPage from './pages/UnitEconomicsPage'
import CommitmentsPage from './pages/CommitmentsPage'
import DriftPage from './pages/DriftPage'

function Sidebar({ isOpen, setIsOpen }: { isOpen: boolean; setIsOpen: (v: boolean) => void }) {
  const location = useLocation()
  const links = [
    { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
    { to: '/costs', icon: DollarSign, label: 'Costs' },
    { to: '/anomalies', icon: AlertTriangle, label: 'Anomalies' },
    { to: '/forecasts', icon: TrendingUp, label: 'Forecasts' },
    { to: '/recommendations', icon: Lightbulb, label: 'Recommendations' },
    { to: '/remediation', icon: Wrench, label: 'Remediation' },
    { to: '/policies', icon: Shield, label: 'Policies' },
    { to: '/chat', icon: MessageSquare, label: 'AI Assistant' },
    { to: '/kubernetes', icon: Container, label: 'Kubernetes' },
    { to: '/unit-economics', icon: Calculator, label: 'Unit Economics' },
    { to: '/commitments', icon: Wallet, label: 'Commitments' },
    { to: '/drift', icon: GitCompare, label: 'Drift Detection' },
    { to: '/settings', icon: Settings, label: 'Settings' },
  ]

  return (
    <>
      <div className={`fixed inset-0 bg-black/50 z-40 lg:hidden ${isOpen ? 'block' : 'hidden'}`} onClick={() => setIsOpen(false)} />
      <aside className={`fixed lg:static inset-y-0 left-0 z-50 w-64 bg-gray-900 text-white transform transition-transform lg:transform-none ${isOpen ? 'translate-x-0' : '-translate-x-full'}`}>
        <div className="p-6 border-b border-gray-800">
          <h1 className="text-xl font-bold text-blue-400">FinOpsMind</h1>
          <p className="text-xs text-gray-400 mt-1">Cloud Cost Intelligence</p>
        </div>
        <nav className="p-4 space-y-1">
          {links.map(({ to, icon: Icon, label }) => (
            <Link key={to} to={to} onClick={() => setIsOpen(false)}
              className={`flex items-center gap-3 px-4 py-3 rounded-lg transition ${location.pathname === to ? 'bg-blue-600 text-white' : 'text-gray-300 hover:bg-gray-800'}`}>
              <Icon size={20} />
              <span>{label}</span>
            </Link>
          ))}
        </nav>
      </aside>
    </>
  )
}

function AuthRedirect() {
  const { isAuthenticated, isLoading } = useAuth()
  if (isLoading) return <LoadingSpinner />
  if (isAuthenticated) return <Navigate to="/" replace />
  return <LoginPage />
}

function ProtectedLayout() {
  const { isAuthenticated, isLoading } = useAuth()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  if (isLoading) return <LoadingSpinner />
  if (!isAuthenticated) return <Navigate to="/login" replace />

  return (
    <div className="flex h-screen bg-gray-50">
      <Sidebar isOpen={sidebarOpen} setIsOpen={setSidebarOpen} />
      <div className="flex-1 flex flex-col overflow-hidden">
        <header className="bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between lg:hidden">
          <button onClick={() => setSidebarOpen(true)} className="p-2 rounded-lg hover:bg-gray-100">
            <Menu size={24} />
          </button>
          <h1 className="font-bold text-blue-600">FinOpsMind</h1>
          <div className="w-10" />
        </header>
        <main className="flex-1 overflow-auto p-6">
          <Routes>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/costs" element={<CostsPage />} />
            <Route path="/anomalies" element={<AnomaliesPage />} />
            <Route path="/forecasts" element={<ForecastsPage />} />
            <Route path="/recommendations" element={<RecommendationsPage />} />
            <Route path="/remediation" element={<RemediationPage />} />
            <Route path="/policies" element={<PoliciesPage />} />
            <Route path="/chat" element={<ChatPage />} />
            <Route path="/kubernetes" element={<KubernetesPage />} />
            <Route path="/unit-economics" element={<UnitEconomicsPage />} />
            <Route path="/commitments" element={<CommitmentsPage />} />
            <Route path="/drift" element={<DriftPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<AuthRedirect />} />
          <Route path="/*" element={<ProtectedLayout />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
