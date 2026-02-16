import React from 'react'
import {
  ChevronUp, ChevronDown, RefreshCw, AlertCircle, CheckCircle, XCircle, Clock
} from 'lucide-react'

export function StatCard({ title, value, subtitle, icon: Icon, trend }: {
  title: string; value: string; subtitle?: string; icon: React.ElementType; trend?: number
}) {
  return (
    <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm text-gray-500">{title}</p>
          <p className="text-2xl font-bold mt-1">{value}</p>
          {subtitle && <p className="text-xs text-gray-400 mt-1">{subtitle}</p>}
          {trend !== undefined && (
            <div className={`flex items-center gap-1 mt-2 text-sm ${trend >= 0 ? 'text-red-500' : 'text-green-500'}`}>
              {trend >= 0 ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
              <span>{Math.abs(trend).toFixed(1)}%</span>
            </div>
          )}
        </div>
        <div className="p-3 bg-blue-50 rounded-lg">
          <Icon className="text-blue-600" size={24} />
        </div>
      </div>
    </div>
  )
}

export function SeverityBadge({ severity }: { severity: string }) {
  const colors: Record<string, string> = {
    critical: 'bg-red-100 text-red-800',
    high: 'bg-orange-100 text-orange-800',
    medium: 'bg-yellow-100 text-yellow-800',
    low: 'bg-blue-100 text-blue-800',
  }
  return <span className={`px-2 py-1 rounded-full text-xs font-medium ${colors[severity] || 'bg-gray-100'}`}>{severity}</span>
}

export function StatusBadge({ status }: { status: string }) {
  const config: Record<string, { color: string; icon: React.ElementType }> = {
    open: { color: 'bg-red-100 text-red-800', icon: AlertCircle },
    investigating: { color: 'bg-yellow-100 text-yellow-800', icon: Clock },
    resolved: { color: 'bg-green-100 text-green-800', icon: CheckCircle },
    implemented: { color: 'bg-green-100 text-green-800', icon: CheckCircle },
    dismissed: { color: 'bg-gray-100 text-gray-800', icon: XCircle },
  }
  const { color, icon: Icon } = config[status] || config.open
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium flex items-center gap-1 w-fit ${color}`}>
      <Icon size={12} />
      {status}
    </span>
  )
}

export function ImpactBadge({ impact }: { impact: string }) {
  const colors: Record<string, string> = {
    high: 'bg-green-100 text-green-800',
    medium: 'bg-blue-100 text-blue-800',
    low: 'bg-gray-100 text-gray-800',
  }
  return <span className={`px-2 py-1 rounded-full text-xs font-medium ${colors[impact] || 'bg-gray-100'}`}>{impact} impact</span>
}

export function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center h-64">
      <RefreshCw className="animate-spin text-blue-600" size={32} />
    </div>
  )
}

export function EmptyState({ title, description, icon: Icon }: { title: string; description: string; icon: React.ElementType }) {
  return (
    <div className="flex flex-col items-center justify-center h-64 text-gray-500">
      <Icon size={48} className="mb-4 text-gray-300" />
      <p className="font-medium">{title}</p>
      <p className="text-sm">{description}</p>
    </div>
  )
}
