'use client';

export default function ObservabilityPage() {
  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-2">Observability</h1>
      <p className="text-slate-400 mb-8">System metrics and logs</p>

      {/* Metrics Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <MetricCard label="Search P95" value="187ms" status="good" />
        <MetricCard label="Index Rate" value="52 MB/s" status="good" />
        <MetricCard label="GPU Memory" value="4.2 GB" status="warning" />
        <MetricCard label="Active Connections" value="12" status="good" />
      </div>

      {/* Charts placeholder */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Search Latency</h3>
          <div className="h-48 flex items-center justify-center text-slate-500 border border-dashed border-slate-600 rounded-lg">
            📊 Prometheus integration pending
          </div>
        </div>
        <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
          <h3 className="text-lg font-semibold text-white mb-4">Indexing Throughput</h3>
          <div className="h-48 flex items-center justify-center text-slate-500 border border-dashed border-slate-600 rounded-lg">
            📈 Metrics visualization pending
          </div>
        </div>
      </div>

      {/* Audit Log Preview */}
      <div className="bg-slate-800 rounded-xl p-6 border border-slate-700">
        <h3 className="text-lg font-semibold text-white mb-4">Recent Activity</h3>
        <div className="space-y-3">
          <LogEntry time="14:32:01" action="Config updated" user="admin" />
          <LogEntry time="14:28:45" action="Index rebuild started" user="system" />
          <LogEntry time="13:55:12" action="User login" user="admin" />
          <LogEntry time="12:40:33" action="Model activated: splade-v3" user="admin" />
        </div>
      </div>
    </div>
  );
}

function MetricCard({ label, value, status }: { label: string; value: string; status: 'good' | 'warning' | 'error' }) {
  const colorClass = {
    good: 'text-green-400',
    warning: 'text-yellow-400',
    error: 'text-red-400',
  }[status];

  return (
    <div className="bg-slate-800 rounded-xl p-4 border border-slate-700">
      <p className="text-slate-400 text-sm">{label}</p>
      <p className={`text-2xl font-bold ${colorClass}`}>{value}</p>
    </div>
  );
}

function LogEntry({ time, action, user }: { time: string; action: string; user: string }) {
  return (
    <div className="flex items-center gap-4 text-sm">
      <span className="text-slate-500 font-mono">{time}</span>
      <span className="text-white">{action}</span>
      <span className="text-slate-400">by {user}</span>
    </div>
  );
}
