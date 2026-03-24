import { useState, useEffect, useCallback } from 'react'
import { useStatus } from '../App'
import SlackSetupModal from './SlackSetupModal'

interface Connection {
  id: string
  provider_config_key: string
  connection_id: string
}

interface Integration {
  name: string
  label: string
  providerKey: string
}

const INTEGRATIONS: Integration[] = [
  { name: 'sheets', label: 'Google Sheets', providerKey: 'google-sheet' },
  { name: 'docs', label: 'Google Docs', providerKey: 'google-docs' },
  { name: 'drive', label: 'Google Drive', providerKey: 'google-drive' },
  { name: 'calendar', label: 'Google Calendar', providerKey: 'google-calendar' },
  { name: 'jira', label: 'Jira', providerKey: 'jira' },
  { name: 'confluence', label: 'Confluence', providerKey: 'confluence' },
  { name: 'google-mail', label: 'Gmail', providerKey: 'google-mail' },
  { name: 'slack', label: 'Slack User', providerKey: 'slack' },
]

function IntegrationBar() {
  const status = useStatus()
  const [connections, setConnections] = useState<Connection[]>([])
  const [showSlackModal, setShowSlackModal] = useState(false)
  const [localDevError, setLocalDevError] = useState(false)

  const isLocalDevResponse = async (res: Response): Promise<boolean> => {
    if (res.status !== 422) return false
    const data = await res.json().catch(() => null)
    if (data?.local_development) {
      setLocalDevError(true)
      return true
    }
    return false
  }

  const loadConnections = useCallback(async () => {
    try {
      const res = await fetch('/api/connections')
      if (await isLocalDevResponse(res)) return
      if (res.ok) {
        const data = await res.json()
        setConnections(data.connections || [])
      }
    } catch {
      // ignore - server may not be ready
    }
  }, [])

  useEffect(() => { loadConnections() }, [loadConnections])

  const isConnected = (integration: Integration) =>
    connections.some(c => c.provider_config_key === integration.providerKey)

  const handleClick = async (name: string) => {
    try {
      const res = await fetch(`/api/connect/${name}`, { method: 'POST' })
      if (await isLocalDevResponse(res)) return
      const data = await res.json()
      if (data.url) {
        const popup = window.open(data.url, '_blank', 'width=600,height=700')
        const check = setInterval(() => {
          if (popup?.closed) {
            clearInterval(check)
            loadConnections()
          }
        }, 500)
      }
    } catch (err) {
      console.error('Connect failed:', err)
    }
  }

  const connectedCount = INTEGRATIONS.filter(i => isConnected(i)).length + (status.ready ? 1 : 0)
  const total = INTEGRATIONS.length + 1

  return (
    <>
      {showSlackModal && <SlackSetupModal onClose={() => setShowSlackModal(false)} />}
      {localDevError && (
        <div className="flex items-center justify-between px-4 py-2 bg-amber-50 border-b border-amber-200 text-amber-800 text-sm">
          <span>
            Integrations are not available in local development. You can only view integrations on deployed apps.{' '}
            <a href="https://apps.applied.dev/apps/my-apps" target="_blank" rel="noopener noreferrer" className="underline font-medium">
              More info
            </a>
          </span>
          <button onClick={() => setLocalDevError(false)} className="ml-4 text-amber-600 hover:text-amber-800 font-bold">
            ✕
          </button>
        </div>
      )}
      <div className="flex items-center gap-3 px-4 py-2 bg-gray-50 border-b border-gray-200 flex-wrap">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wide whitespace-nowrap">
          Integrations ({connectedCount}/{total})
        </span>
        <div className="flex items-center gap-2 flex-wrap flex-1">
          <button
            onClick={() => setShowSlackModal(true)}
            className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium transition-colors ${
              status.ready
                ? 'bg-gray-800 text-white hover:bg-gray-700'
                : 'bg-gray-200 text-gray-600 hover:bg-gray-300'
            }`}
          >
            <span className={`w-1.5 h-1.5 rounded-full ${status.ready ? 'bg-green-400' : 'bg-gray-400'}`} />
            Slack Bot
          </button>
          {INTEGRATIONS.map(integration => {
            const connected = isConnected(integration)
            return (
              <button
                key={integration.name}
                onClick={() => handleClick(integration.name)}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium transition-colors ${
                  connected
                    ? 'bg-gray-800 text-white hover:bg-gray-700'
                    : 'bg-gray-200 text-gray-600 hover:bg-gray-300'
                }`}
              >
                <span className={`w-1.5 h-1.5 rounded-full ${connected ? 'bg-green-400' : 'bg-gray-400'}`} />
                {integration.label}
              </button>
            )
          })}
        </div>
        <a
          href="https://grid-appliedint.enterprise.slack.com/archives/C0A9VNNM96W"
          target="_blank"
          rel="noopener noreferrer"
          className="ml-auto flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-700 hover:bg-blue-200 transition-colors whitespace-nowrap"
        >
          Need Help?
        </a>
      </div>
    </>
  )
}

export default IntegrationBar
