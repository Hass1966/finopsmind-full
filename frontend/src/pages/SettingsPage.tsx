import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'
import type { Provider, OrgSettings, AWSCredentialsInput, AzureCredentialsInput } from '../lib/types'
import { LoadingSpinner } from '../components/shared'
import { useAuth } from '../contexts/AuthContext'

const AWS_REGIONS = [
  'us-east-1', 'us-east-2', 'us-west-1', 'us-west-2',
  'eu-west-1', 'eu-west-2', 'eu-central-1',
  'ap-southeast-1', 'ap-southeast-2', 'ap-northeast-1',
]

export default function SettingsPage() {
  const queryClient = useQueryClient()
  const { user, logout } = useAuth()
  const isAdmin = user?.role === 'admin'

  const { data: providers, isLoading } = useQuery<Provider[]>({
    queryKey: ['providers'],
    queryFn: () => api.get('/providers')
  })

  const { data: settingsData } = useQuery<{ organization: string; settings: OrgSettings }>({
    queryKey: ['settings'],
    queryFn: () => api.get('/settings')
  })

  const [orgName, setOrgName] = useState('')
  const [settings, setSettings] = useState<OrgSettings>({
    default_currency: 'USD',
    timezone: 'UTC',
    fiscal_year_start: 1,
    alerts_enabled: true,
  })
  const [saveStatus, setSaveStatus] = useState<string | null>(null)

  // Cloud provider form state
  const [showAwsForm, setShowAwsForm] = useState(false)
  const [showAzureForm, setShowAzureForm] = useState(false)
  const [awsCreds, setAwsCreds] = useState<AWSCredentialsInput>({
    access_key_id: '', secret_key: '', region: 'us-east-1', assume_role_arn: '', external_id: ''
  })
  const [azureCreds, setAzureCreds] = useState<AzureCredentialsInput>({
    tenant_id: '', client_id: '', client_secret: '', subscription_id: ''
  })
  const [providerStatus, setProviderStatus] = useState<string | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null)

  useEffect(() => {
    if (settingsData) {
      setOrgName(settingsData.organization)
      setSettings(settingsData.settings)
    }
  }, [settingsData])

  const updateMutation = useMutation({
    mutationFn: (data: { name?: string; settings?: OrgSettings }) =>
      api.put('/settings', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      setSaveStatus('Settings saved successfully')
      setTimeout(() => setSaveStatus(null), 3000)
    },
    onError: () => {
      setSaveStatus('Failed to save settings')
      setTimeout(() => setSaveStatus(null), 3000)
    }
  })

  const createProviderMutation = useMutation({
    mutationFn: (data: { provider_type: string; name: string; credentials: AWSCredentialsInput | AzureCredentialsInput }) =>
      api.post<{ id: string }>('/providers', data),
    onSuccess: async (data) => {
      setProviderStatus('Provider created. Testing connection...')
      try {
        const testResult = await api.post<{ healthy: boolean; message: string }>(`/providers/${data.id}/test`, {})
        if (testResult.healthy) {
          setProviderStatus('Connected! Starting initial sync...')
          await api.post(`/providers/${data.id}/sync`, {})
          setProviderStatus('Sync complete!')
        } else {
          setProviderStatus(`Connection test failed: ${testResult.message}`)
        }
      } catch (e: any) {
        setProviderStatus(`Test failed: ${e.message}`)
      }
      queryClient.invalidateQueries({ queryKey: ['providers'] })
      queryClient.invalidateQueries({ queryKey: ['costSummary'] })
      queryClient.invalidateQueries({ queryKey: ['costTrend'] })
      setShowAwsForm(false)
      setShowAzureForm(false)
      setTimeout(() => setProviderStatus(null), 5000)
    },
    onError: (e: any) => {
      setProviderStatus(`Failed: ${e.message}`)
      setTimeout(() => setProviderStatus(null), 5000)
    }
  })

  const deleteProviderMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/providers/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['providers'] })
      setDeleteConfirm(null)
      setProviderStatus('Provider disconnected')
      setTimeout(() => setProviderStatus(null), 3000)
    }
  })

  const testProviderMutation = useMutation({
    mutationFn: (id: string) => api.post<{ healthy: boolean; message: string }>(`/providers/${id}/test`, {}),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['providers'] })
      setProviderStatus(data.healthy ? 'Connection healthy!' : `Unhealthy: ${data.message}`)
      setTimeout(() => setProviderStatus(null), 3000)
    }
  })

  const syncProviderMutation = useMutation({
    mutationFn: (id: string) => api.post<{ records: number }>(`/providers/${id}/sync`, {}),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['providers'] })
      queryClient.invalidateQueries({ queryKey: ['costSummary'] })
      queryClient.invalidateQueries({ queryKey: ['costTrend'] })
      setProviderStatus(`Sync complete: ${data.records} records`)
      setTimeout(() => setProviderStatus(null), 3000)
    }
  })

  const handleSave = () => {
    updateMutation.mutate({ name: orgName, settings })
  }

  const handleConnectAws = () => {
    if (!awsCreds.access_key_id || !awsCreds.secret_key || !awsCreds.region) return
    createProviderMutation.mutate({
      provider_type: 'aws',
      name: 'AWS',
      credentials: awsCreds,
    })
  }

  const handleConnectAzure = () => {
    if (!azureCreds.tenant_id || !azureCreds.client_id || !azureCreds.client_secret || !azureCreds.subscription_id) return
    createProviderMutation.mutate({
      provider_type: 'azure',
      name: 'Azure',
      credentials: azureCreds,
    })
  }

  const hasAws = providers?.some(p => p.provider_type === 'aws')
  const hasAzure = providers?.some(p => p.provider_type === 'azure')

  if (isLoading) return <LoadingSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Settings</h1>
          <p className="text-gray-500 text-sm mt-1">Configure your FinOpsMind platform</p>
        </div>
        {user && (
          <div className="flex items-center gap-4">
            <span className="text-sm text-gray-500">{user.email}</span>
            <button onClick={logout} className="px-4 py-2 text-sm text-red-600 border border-red-200 rounded-lg hover:bg-red-50">
              Sign Out
            </button>
          </div>
        )}
      </div>

      {saveStatus && (
        <div className={`p-3 rounded-lg text-sm ${saveStatus.includes('success') ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'}`}>
          {saveStatus}
        </div>
      )}

      {providerStatus && (
        <div className={`p-3 rounded-lg text-sm ${providerStatus.includes('fail') || providerStatus.includes('Failed') || providerStatus.includes('Unhealthy') ? 'bg-red-50 text-red-700 border border-red-200' : 'bg-blue-50 text-blue-700 border border-blue-200'}`}>
          {providerStatus}
        </div>
      )}

      {/* Organization */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Organization</h2>
        </div>
        <div className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Organization Name</label>
            <input type="text" value={orgName} onChange={e => setOrgName(e.target.value)}
              className="w-full max-w-md px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Default Currency</label>
              <select value={settings.default_currency} onChange={e => setSettings({ ...settings, default_currency: e.target.value })}
                className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm">
                <option value="USD">USD</option>
                <option value="EUR">EUR</option>
                <option value="GBP">GBP</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Timezone</label>
              <select value={settings.timezone} onChange={e => setSettings({ ...settings, timezone: e.target.value })}
                className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm">
                <option value="UTC">UTC</option>
                <option value="America/New_York">Eastern</option>
                <option value="America/Chicago">Central</option>
                <option value="America/Los_Angeles">Pacific</option>
                <option value="Europe/London">London</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Fiscal Year Start</label>
              <select value={settings.fiscal_year_start} onChange={e => setSettings({ ...settings, fiscal_year_start: parseInt(e.target.value) })}
                className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm">
                {Array.from({ length: 12 }, (_, i) => (
                  <option key={i + 1} value={i + 1}>{new Date(2000, i).toLocaleString('en-US', { month: 'long' })}</option>
                ))}
              </select>
            </div>
          </div>
        </div>
      </div>

      {/* Cloud Providers */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Cloud Providers</h2>
          <p className="text-sm text-gray-500 mt-1">Connect your cloud accounts to start tracking costs</p>
        </div>

        {/* Connected Providers */}
        {providers && providers.length > 0 && (
          <div className="divide-y divide-gray-100">
            {providers.map(provider => (
              <div key={provider.id} className="p-6">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className={`w-12 h-12 rounded-lg flex items-center justify-center ${
                      provider.provider_type === 'aws' ? 'bg-orange-100' : 'bg-cyan-100'
                    }`}>
                      <span className="text-lg font-bold">
                        {provider.provider_type === 'aws' ? 'AWS' : 'AZ'}
                      </span>
                    </div>
                    <div>
                      <p className="font-medium">{provider.name}</p>
                      <p className="text-sm text-gray-500">
                        Status: {provider.status}
                        {provider.last_sync_at && ` | Last sync: ${new Date(provider.last_sync_at).toLocaleString()}`}
                      </p>
                      {provider.status_message && (
                        <p className="text-xs text-gray-400 mt-0.5">{provider.status_message}</p>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={`px-3 py-1 rounded-full text-xs font-medium ${
                      provider.status === 'connected' ? 'bg-green-100 text-green-800' :
                      provider.status === 'error' ? 'bg-red-100 text-red-800' :
                      'bg-yellow-100 text-yellow-800'
                    }`}>
                      {provider.status === 'connected' ? 'Connected' :
                       provider.status === 'error' ? 'Error' : 'Pending'}
                    </span>
                    {isAdmin && (
                      <>
                        <button onClick={() => testProviderMutation.mutate(provider.id)}
                          disabled={testProviderMutation.isPending}
                          className="px-3 py-1 text-xs border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-50">
                          Test
                        </button>
                        <button onClick={() => syncProviderMutation.mutate(provider.id)}
                          disabled={syncProviderMutation.isPending}
                          className="px-3 py-1 text-xs border border-blue-200 text-blue-600 rounded-lg hover:bg-blue-50 disabled:opacity-50">
                          Sync
                        </button>
                        {deleteConfirm === provider.id ? (
                          <div className="flex items-center gap-1">
                            <button onClick={() => deleteProviderMutation.mutate(provider.id)}
                              className="px-3 py-1 text-xs bg-red-600 text-white rounded-lg hover:bg-red-700">
                              Confirm
                            </button>
                            <button onClick={() => setDeleteConfirm(null)}
                              className="px-3 py-1 text-xs border border-gray-200 rounded-lg hover:bg-gray-50">
                              Cancel
                            </button>
                          </div>
                        ) : (
                          <button onClick={() => setDeleteConfirm(provider.id)}
                            className="px-3 py-1 text-xs text-red-600 border border-red-200 rounded-lg hover:bg-red-50">
                            Disconnect
                          </button>
                        )}
                      </>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Connect buttons */}
        {isAdmin && (
          <div className="p-6 border-t border-gray-100 space-y-4">
            {!hasAws && !showAwsForm && (
              <button onClick={() => setShowAwsForm(true)}
                className="w-full p-4 border-2 border-dashed border-gray-200 rounded-lg text-sm text-gray-500 hover:border-orange-300 hover:text-orange-600 transition">
                + Connect AWS Account
              </button>
            )}
            {!hasAzure && !showAzureForm && (
              <button onClick={() => setShowAzureForm(true)}
                className="w-full p-4 border-2 border-dashed border-gray-200 rounded-lg text-sm text-gray-500 hover:border-cyan-300 hover:text-cyan-600 transition">
                + Connect Azure Account
              </button>
            )}

            {/* AWS Form */}
            {showAwsForm && (
              <div className="border border-orange-200 rounded-lg p-6 bg-orange-50/30 space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold text-orange-800">Connect AWS</h3>
                  <button onClick={() => setShowAwsForm(false)} className="text-sm text-gray-500 hover:text-gray-700">Cancel</button>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Access Key ID *</label>
                    <input type="text" value={awsCreds.access_key_id}
                      onChange={e => setAwsCreds({ ...awsCreds, access_key_id: e.target.value })}
                      placeholder="AKIAIOSFODNN7EXAMPLE"
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-orange-500" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Secret Access Key *</label>
                    <input type="password" value={awsCreds.secret_key}
                      onChange={e => setAwsCreds({ ...awsCreds, secret_key: e.target.value })}
                      placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-orange-500" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Region *</label>
                    <select value={awsCreds.region}
                      onChange={e => setAwsCreds({ ...awsCreds, region: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-orange-500">
                      {AWS_REGIONS.map(r => <option key={r} value={r}>{r}</option>)}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Assume Role ARN (optional)</label>
                    <input type="text" value={awsCreds.assume_role_arn || ''}
                      onChange={e => setAwsCreds({ ...awsCreds, assume_role_arn: e.target.value })}
                      placeholder="arn:aws:iam::123456789012:role/FinOpsMindRole"
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-orange-500" />
                  </div>
                </div>
                <button onClick={handleConnectAws}
                  disabled={createProviderMutation.isPending || !awsCreds.access_key_id || !awsCreds.secret_key}
                  className="px-6 py-2 bg-orange-600 text-white rounded-lg text-sm font-medium hover:bg-orange-700 disabled:opacity-50 transition">
                  {createProviderMutation.isPending ? 'Connecting...' : 'Connect AWS'}
                </button>
              </div>
            )}

            {/* Azure Form */}
            {showAzureForm && (
              <div className="border border-cyan-200 rounded-lg p-6 bg-cyan-50/30 space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold text-cyan-800">Connect Azure</h3>
                  <button onClick={() => setShowAzureForm(false)} className="text-sm text-gray-500 hover:text-gray-700">Cancel</button>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Tenant ID *</label>
                    <input type="text" value={azureCreds.tenant_id}
                      onChange={e => setAzureCreds({ ...azureCreds, tenant_id: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-cyan-500" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Client ID *</label>
                    <input type="text" value={azureCreds.client_id}
                      onChange={e => setAzureCreds({ ...azureCreds, client_id: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-cyan-500" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Client Secret *</label>
                    <input type="password" value={azureCreds.client_secret}
                      onChange={e => setAzureCreds({ ...azureCreds, client_secret: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-cyan-500" />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Subscription ID *</label>
                    <input type="text" value={azureCreds.subscription_id}
                      onChange={e => setAzureCreds({ ...azureCreds, subscription_id: e.target.value })}
                      className="w-full px-4 py-2 border border-gray-200 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-cyan-500" />
                  </div>
                </div>
                <button onClick={handleConnectAzure}
                  disabled={createProviderMutation.isPending || !azureCreds.tenant_id || !azureCreds.client_id || !azureCreds.client_secret || !azureCreds.subscription_id}
                  className="px-6 py-2 bg-cyan-600 text-white rounded-lg text-sm font-medium hover:bg-cyan-700 disabled:opacity-50 transition">
                  {createProviderMutation.isPending ? 'Connecting...' : 'Connect Azure'}
                </button>
              </div>
            )}
          </div>
        )}

        {!isAdmin && !providers?.length && (
          <div className="p-6 text-center text-gray-500">No providers configured. Ask your admin to connect a cloud account.</div>
        )}
      </div>

      {/* Notifications */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-semibold">Notifications</h2>
        </div>
        <div className="p-6">
          <div className="flex items-center justify-between py-3">
            <div>
              <p className="font-medium">Enable Alerts</p>
              <p className="text-sm text-gray-500">Receive anomaly, budget, and recommendation alerts</p>
            </div>
            <button onClick={() => setSettings({ ...settings, alerts_enabled: !settings.alerts_enabled })}
              className={`w-12 h-6 rounded-full transition-colors ${settings.alerts_enabled ? 'bg-blue-600' : 'bg-gray-200'}`}>
              <div className={`w-5 h-5 bg-white rounded-full shadow transition-transform ${settings.alerts_enabled ? 'translate-x-6' : 'translate-x-0.5'}`} />
            </button>
          </div>
        </div>
      </div>

      {/* Save Button */}
      <div className="flex justify-end">
        <button onClick={handleSave} disabled={updateMutation.isPending}
          className="px-6 py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 transition">
          {updateMutation.isPending ? 'Saving...' : 'Save Settings'}
        </button>
      </div>

      {/* About */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        <div className="p-6">
          <h2 className="text-lg font-semibold mb-4">About FinOpsMind</h2>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div><p className="text-gray-500">Version</p><p className="font-medium">1.0.0</p></div>
            <div><p className="text-gray-500">ML Model</p><p className="font-medium">Prophet + Isolation Forest</p></div>
            <div><p className="text-gray-500">Backend</p><p className="font-medium">Go 1.23</p></div>
            <div><p className="text-gray-500">Frontend</p><p className="font-medium">React 18 + TypeScript</p></div>
          </div>
        </div>
      </div>
    </div>
  )
}
