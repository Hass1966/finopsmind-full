import { useQuery } from '@tanstack/react-query'
import {
  DollarSign, Server, Layers, Gauge, ArrowDownRight
} from 'lucide-react'
import {
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar
} from 'recharts'
import { api } from '../lib/api'
import { StatCard, LoadingSpinner } from '../components/shared'

interface Cluster {
  cluster_name: string
  provider: string
  region: string
  node_count: number
  pod_count: number
  total_cost: number
  cpu_cost: number
  memory_cost: number
  storage_cost: number
  efficiency: number
  namespaces: number
}

interface Namespace {
  namespace: string
  pods: number
  cpu_requests: string
  memory_requests: string
  cost: number
  efficiency: number
}

interface Rightsizing {
  namespace: string
  deployment: string
  current_cpu: string
  recommended_cpu: string
  current_memory: string
  recommended_memory: string
  avg_cpu_usage: number
  avg_memory_usage: number
  monthly_savings: number
}

export default function KubernetesPage() {
  const { data: clustersData, isLoading: loadingClusters } = useQuery<{ data: Cluster[]; total_cost: number }>({
    queryKey: ['kubernetes-clusters'],
    queryFn: () => api.get('/kubernetes/clusters')
  })

  const { data: namespacesData, isLoading: loadingNamespaces } = useQuery<{ data: Namespace[] }>({
    queryKey: ['kubernetes-namespaces'],
    queryFn: () => api.get('/kubernetes/namespaces')
  })

  const { data: rightsizingData, isLoading: loadingRightsizing } = useQuery<{ data: Rightsizing[]; total_monthly_savings: number }>({
    queryKey: ['kubernetes-rightsizing'],
    queryFn: () => api.get('/kubernetes/rightsizing')
  })

  const isLoading = loadingClusters || loadingNamespaces || loadingRightsizing

  const clusters = clustersData?.data || []
  const namespaces = namespacesData?.data || []
  const rightsizing = rightsizingData?.data || []
  const totalCost = clustersData?.total_cost || 0
  const totalSavings = rightsizingData?.total_monthly_savings || 0

  const totalNamespaces = clusters.reduce((sum, c) => sum + c.namespaces, 0)
  const avgEfficiency = clusters.length > 0
    ? clusters.reduce((sum, c) => sum + c.efficiency, 0) / clusters.length
    : 0

  if (isLoading) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Kubernetes Cost Allocation</h1>
        <p className="text-gray-500 text-sm mt-1">Cluster, namespace, and pod-level cost visibility</p>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard title="Total K8s Cost" value={`$${totalCost.toLocaleString()}/mo`} icon={DollarSign} />
        <StatCard title="Clusters" value={String(clusters.length)} icon={Server} />
        <StatCard title="Namespaces" value={String(totalNamespaces)} icon={Layers} />
        <StatCard title="Avg Efficiency" value={`${avgEfficiency.toFixed(1)}%`} icon={Gauge} />
      </div>

      {/* Cluster Overview Table */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Cluster Overview</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Cluster</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Provider</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Region</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Nodes</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Pods</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">CPU Cost</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Memory Cost</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Storage Cost</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Total Cost</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Efficiency</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {clusters.map(cluster => (
                <tr key={cluster.cluster_name} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <p className="font-medium text-sm">{cluster.cluster_name}</p>
                  </td>
                  <td className="px-6 py-4 text-sm uppercase text-gray-600">{cluster.provider}</td>
                  <td className="px-6 py-4 text-sm text-gray-600">{cluster.region}</td>
                  <td className="px-6 py-4 text-sm text-right">{cluster.node_count}</td>
                  <td className="px-6 py-4 text-sm text-right">{cluster.pod_count}</td>
                  <td className="px-6 py-4 text-sm text-right">${cluster.cpu_cost.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-right">${cluster.memory_cost.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-right">${cluster.storage_cost.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-right font-semibold">${cluster.total_cost.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-right">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                      cluster.efficiency >= 75 ? 'bg-green-100 text-green-800' :
                      cluster.efficiency >= 60 ? 'bg-yellow-100 text-yellow-800' :
                      'bg-red-100 text-red-800'
                    }`}>
                      {cluster.efficiency}%
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Namespace Cost Breakdown Chart */}
      <div className="bg-white rounded-xl shadow-sm p-6 border border-gray-100">
        <h2 className="text-lg font-semibold mb-4">Namespace Cost Breakdown</h2>
        <ResponsiveContainer width="100%" height={350}>
          <BarChart
            layout="vertical"
            data={namespaces.map(ns => ({ name: ns.namespace, cost: ns.cost, efficiency: ns.efficiency }))}
            margin={{ left: 20 }}
          >
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis type="number" tick={{ fontSize: 12 }} tickFormatter={(v) => `$${v}`} />
            <YAxis type="category" dataKey="name" tick={{ fontSize: 12 }} width={120} />
            <Tooltip
              formatter={(v: number, name: string) =>
                name === 'cost' ? `$${v.toLocaleString()}` : `${v}%`
              }
            />
            <Bar dataKey="cost" fill="#6366f1" radius={[0, 4, 4, 0]} name="cost" />
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* Namespace Details Table */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Namespace Details</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Namespace</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Pods</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">CPU Requests</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Memory Requests</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Monthly Cost</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Efficiency</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {namespaces.map(ns => (
                <tr key={ns.namespace} className="hover:bg-gray-50">
                  <td className="px-6 py-4 font-medium text-sm">{ns.namespace}</td>
                  <td className="px-6 py-4 text-sm text-right">{ns.pods}</td>
                  <td className="px-6 py-4 text-sm text-right text-gray-600">{ns.cpu_requests}</td>
                  <td className="px-6 py-4 text-sm text-right text-gray-600">{ns.memory_requests}</td>
                  <td className="px-6 py-4 text-sm text-right font-semibold">${ns.cost.toLocaleString()}</td>
                  <td className="px-6 py-4 text-sm text-right">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                      ns.efficiency >= 80 ? 'bg-green-100 text-green-800' :
                      ns.efficiency >= 60 ? 'bg-yellow-100 text-yellow-800' :
                      'bg-red-100 text-red-800'
                    }`}>
                      {ns.efficiency}%
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pod Rightsizing Recommendations */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Pod Rightsizing Recommendations</h2>
            <p className="text-sm text-gray-500 mt-1">Reduce over-provisioned resources to save costs</p>
          </div>
          <div className="flex items-center gap-2 text-green-600 bg-green-50 px-4 py-2 rounded-lg">
            <ArrowDownRight size={18} />
            <span className="font-semibold">${totalSavings.toLocaleString()}/mo savings</span>
          </div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Namespace</th>
                <th className="text-left px-6 py-4 text-sm font-medium text-gray-500">Deployment</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Current CPU</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Rec. CPU</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Avg CPU %</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Current Mem</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Rec. Mem</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Avg Mem %</th>
                <th className="text-right px-6 py-4 text-sm font-medium text-gray-500">Monthly Savings</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rightsizing.map(rec => (
                <tr key={`${rec.namespace}-${rec.deployment}`} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm text-gray-600">{rec.namespace}</td>
                  <td className="px-6 py-4 text-sm font-medium font-mono">{rec.deployment}</td>
                  <td className="px-6 py-4 text-sm text-right text-gray-600">{rec.current_cpu}</td>
                  <td className="px-6 py-4 text-sm text-right text-blue-600 font-medium">{rec.recommended_cpu}</td>
                  <td className="px-6 py-4 text-sm text-right">
                    <span className={`${rec.avg_cpu_usage < 30 ? 'text-red-600' : 'text-gray-600'}`}>
                      {rec.avg_cpu_usage}%
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-right text-gray-600">{rec.current_memory}</td>
                  <td className="px-6 py-4 text-sm text-right text-blue-600 font-medium">{rec.recommended_memory}</td>
                  <td className="px-6 py-4 text-sm text-right">
                    <span className={`${rec.avg_memory_usage < 30 ? 'text-red-600' : 'text-gray-600'}`}>
                      {rec.avg_memory_usage}%
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-right text-green-600 font-semibold">
                    ${rec.monthly_savings.toLocaleString()}/mo
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
