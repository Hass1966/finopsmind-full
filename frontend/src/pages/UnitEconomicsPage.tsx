import { useQuery } from '@tanstack/react-query'
import {
  Users, Zap, ArrowRightLeft, Database, DollarSign,
  TrendingUp, TrendingDown, Minus
} from 'lucide-react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer
} from 'recharts'
import { api } from '../lib/api'
import { LoadingSpinner, EmptyState } from '../components/shared'

interface MetricHistory {
  month: string
  value: number
}

interface UnitMetric {
  name: string
  current_value: number
  previous_value: number
  change_pct: number
  unit: string
  trend: 'improving' | 'degrading' | 'stable'
  history: MetricHistory[]
}

interface UnitEconomicsResponse {
  metrics: UnitMetric[]
  summary: {
    total_active_users: number
    total_api_calls: string
    total_transactions: string
    total_storage_gb: number
    infrastructure_cost: number
  }
}

const trendConfig = {
  improving: { color: 'text-green-600', bg: 'bg-green-50', Icon: TrendingDown, label: 'Improving' },
  degrading: { color: 'text-red-600', bg: 'bg-red-50', Icon: TrendingUp, label: 'Degrading' },
  stable:    { color: 'text-gray-600', bg: 'bg-gray-50', Icon: Minus, label: 'Stable' },
}

const chartColors = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6']

function formatMetricValue(value: number): string {
  if (value >= 1) return `$${value.toFixed(2)}`
  if (value >= 0.001) return `$${value.toFixed(4)}`
  return `$${value.toFixed(6)}`
}

export default function UnitEconomicsPage() {
  const { data, isLoading } = useQuery<UnitEconomicsResponse>({
    queryKey: ['unit-economics'],
    queryFn: () => api.get('/unit-economics'),
  })

  if (isLoading) return <LoadingSpinner />

  if (!data?.metrics?.length) {
    return <EmptyState title="No unit economics data" description="Data will appear once cost and usage metrics are collected" icon={DollarSign} />
  }

  const { metrics, summary } = data

  const summaryCards = [
    { title: 'Active Users', value: summary.total_active_users.toLocaleString(), icon: Users },
    { title: 'API Calls', value: summary.total_api_calls, icon: Zap },
    { title: 'Transactions', value: summary.total_transactions, icon: ArrowRightLeft },
    { title: 'Storage', value: `${summary.total_storage_gb.toLocaleString()} GB`, icon: Database },
    { title: 'Infra Cost', value: `$${summary.infrastructure_cost.toLocaleString()}`, icon: DollarSign },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Unit Economics</h1>
        <p className="text-gray-500 text-sm mt-1">Cost-per-business-metric visibility across your infrastructure</p>
      </div>

      {/* Summary Row */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        {summaryCards.map((card) => (
          <div key={card.title} className="bg-white rounded-xl shadow-sm p-4 border border-gray-100">
            <div className="flex items-center gap-2 text-gray-500 mb-1">
              <card.icon size={16} />
              <span className="text-xs font-medium">{card.title}</span>
            </div>
            <p className="text-lg font-bold">{card.value}</p>
          </div>
        ))}
      </div>

      {/* Metric Cards + Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {metrics.map((metric, idx) => {
          const { color, bg, Icon, label } = trendConfig[metric.trend]
          const changePct = metric.change_pct

          return (
            <div key={metric.name} className="bg-white rounded-xl shadow-sm border border-gray-100 p-6">
              {/* Header */}
              <div className="flex items-start justify-between mb-4">
                <div>
                  <h3 className="font-semibold text-gray-900">{metric.name}</h3>
                  <p className="text-2xl font-bold mt-1">{formatMetricValue(metric.current_value)}</p>
                </div>
                <div className="flex flex-col items-end gap-1">
                  <span className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium ${bg} ${color}`}>
                    <Icon size={14} />
                    {label}
                  </span>
                  <span className={`text-sm font-medium ${changePct < 0 ? 'text-green-600' : changePct > 0 ? 'text-red-600' : 'text-gray-500'}`}>
                    {changePct > 0 ? '+' : ''}{changePct.toFixed(1)}%
                  </span>
                </div>
              </div>

              {/* Previous value */}
              <p className="text-xs text-gray-400 mb-4">
                Previous: {formatMetricValue(metric.previous_value)}
              </p>

              {/* Line Chart */}
              <ResponsiveContainer width="100%" height={180}>
                <LineChart data={metric.history}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" />
                  <XAxis
                    dataKey="month"
                    tick={{ fontSize: 11 }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <YAxis
                    tick={{ fontSize: 11 }}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v: number) => formatMetricValue(v)}
                    width={70}
                  />
                  <Tooltip
                    formatter={(v: number) => [formatMetricValue(v), 'Cost']}
                    labelStyle={{ fontWeight: 600 }}
                  />
                  <Line
                    type="monotone"
                    dataKey="value"
                    stroke={chartColors[idx % chartColors.length]}
                    strokeWidth={2}
                    dot={{ r: 3, fill: chartColors[idx % chartColors.length] }}
                    activeDot={{ r: 5 }}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )
        })}
      </div>
    </div>
  )
}
