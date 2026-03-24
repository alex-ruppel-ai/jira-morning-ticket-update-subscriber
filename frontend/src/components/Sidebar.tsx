import { Tab } from '../App'

interface SidebarProps {
  activeTab: Tab
  setActiveTab: (tab: Tab) => void
  isOpen: boolean
  onToggle: () => void
}

const tabs: { id: Tab; label: string; icon: string }[] = [
  { id: 'home', label: 'Home', icon: '🏠' },
  { id: 'daily-update', label: 'Daily Update', icon: '📋' },
]

function ChevronRight() {
  return (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
    </svg>
  )
}

function ChevronLeft() {
  return (
    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
    </svg>
  )
}

function Sidebar({ activeTab, setActiveTab, isOpen, onToggle }: SidebarProps) {
  if (!isOpen) {
    return (
      <aside className="w-10 bg-white shadow-md flex flex-col items-center pt-4 shrink-0">
        <button onClick={onToggle} className="bg-blue-600 hover:bg-blue-700 text-white p-1 rounded">
          <ChevronRight />
        </button>
      </aside>
    )
  }

  return (
    <aside className="w-64 bg-white shadow-md flex flex-col shrink-0">
      <div className="p-6 border-b flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-gray-800">Agentic App Template</h1>
          <p className="text-sm text-gray-500 mt-1">Go + React</p>
        </div>
        <button onClick={onToggle} className="bg-blue-600 hover:bg-blue-700 text-white p-1 ml-2 shrink-0 rounded">
          <ChevronLeft />
        </button>
      </div>

<nav className="flex-1 p-4">
        <ul className="space-y-2">
          {tabs.map((tab) => (
            <li key={tab.id}>
              <button
                onClick={() => setActiveTab(tab.id)}
                className={`w-full text-left px-4 py-3 rounded-lg transition-colors ${
                  activeTab === tab.id
                    ? 'bg-blue-100 text-blue-700 font-medium'
                    : 'text-gray-600 hover:bg-gray-100'
                }`}
              >
                <span className="mr-3">{tab.icon}</span>
                {tab.label}
              </button>
            </li>
          ))}
        </ul>
      </nav>
    </aside>
  )
}

export default Sidebar