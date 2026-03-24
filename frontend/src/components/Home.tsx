function Home() {
  return (
    <div className="max-w-5xl mx-auto mt-12 px-4">
      <div className="bg-white rounded-xl shadow p-8">
        <h1 className="text-3xl font-bold text-gray-900 mb-4">
          Welcome to the Agentic App Template
        </h1>

        <p className="text-gray-600 text-lg mb-8">
          A blank canvas for building Slack bots and integrated apps with Go + React.
        </p>

        <section className="mb-8">
          <h2 className="text-xl font-semibold text-gray-800 mb-3">Getting Started</h2>
          <div className="space-y-3 text-gray-700">
            <p>
              This template provides a minimal foundation for building apps on the Apps Platform.
              Start by asking Claude Code to add features — it will guide you through:
            </p>
            <ul className="list-disc list-inside space-y-2 ml-4">
              <li>Slack bot interactions (mentions, commands, buttons, modals)</li>
              <li>Third-party integrations (Google Sheets, Jira, Confluence, etc.)</li>
              <li>Database persistence with MySQL</li>
              <li>Frontend components with React + TypeScript</li>
            </ul>
          </div>
        </section>

        <section className="mb-8">
          <h2 className="text-xl font-semibold text-gray-800 mb-3">Available Integrations</h2>
          <p className="text-gray-600 mb-4">
            Claude Code knows how to wire up these integrations. Just ask to add one:
          </p>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">💬</div>
              <div className="font-medium text-sm">Slack Bot</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">👤</div>
              <div className="font-medium text-sm">Slack User</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">📊</div>
              <div className="font-medium text-sm">Google Sheets</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">📄</div>
              <div className="font-medium text-sm">Google Docs</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">📁</div>
              <div className="font-medium text-sm">Google Drive</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">📅</div>
              <div className="font-medium text-sm">Google Calendar</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">✉️</div>
              <div className="font-medium text-sm">Gmail</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">🎫</div>
              <div className="font-medium text-sm">Jira</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">📚</div>
              <div className="font-medium text-sm">Confluence</div>
            </div>
            <div className="border border-gray-200 rounded-lg p-3 text-center">
              <div className="text-2xl mb-1">👥</div>
              <div className="font-medium text-sm">Anaheim</div>
            </div>
          </div>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-gray-800 mb-3">What to Build</h2>
          <p className="text-gray-600 mb-4">Some ideas to get started:</p>
          <ul className="list-disc list-inside space-y-2 text-gray-700 ml-4">
            <li>A Slack bot that answers questions about your Google Docs</li>
            <li>A dashboard showing upcoming calendar events</li>
            <li>A tool to create Jira tickets from Slack messages</li>
            <li>An email summarizer that posts daily Gmail digests to Slack</li>
            <li>A team directory powered by Anaheim</li>
          </ul>
        </section>
      </div>
    </div>
  )
}

export default Home