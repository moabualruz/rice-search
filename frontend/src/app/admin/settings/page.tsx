'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';

const API_BASE = 'http://localhost:8000/api/v1';

interface SettingsData {
  settings: Record<string, any>;
  count: number;
  version: number;
}

interface SettingValue {
  key: string;
  value: any;
  type: 'string' | 'number' | 'boolean' | 'array' | 'object';
  modified?: boolean;
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<Record<string, any>>({});
  const [originalSettings, setOriginalSettings] = useState<Record<string, any>>({});
  const [version, setVersion] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const fetchSettings = async () => {
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/settings/`);
      if (res.ok) {
        const data: SettingsData = await res.json();
        setSettings(data.settings);
        setOriginalSettings(JSON.parse(JSON.stringify(data.settings)));
        setVersion(data.version);
      }
    } catch (e) {
      console.error('Failed to fetch settings:', e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchSettings();
  }, []);

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const updateSetting = async (key: string, value: any) => {
    try {
      const res = await fetch(`${API_BASE}/settings/${key}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ value })
      });

      if (res.ok) {
        const result = await res.json();
        setVersion(result.version);

        // Update local state
        const keys = key.split('.');
        setSettings(prev => {
          const newSettings = { ...prev };
          let current: any = newSettings;
          for (let i = 0; i < keys.length - 1; i++) {
            current = current[keys[i]];
          }
          current[keys[keys.length - 1]] = value;
          return newSettings;
        });

        showMessage('success', `Setting ${key} updated`);
      } else {
        showMessage('error', `Failed to update ${key}`);
      }
    } catch (e) {
      showMessage('error', `Error updating ${key}`);
    }
  };

  const saveBulk = async () => {
    setSaving(true);
    try {
      const changedSettings: Record<string, any> = {};

      // Find all changed settings
      Object.keys(settings).forEach(key => {
        if (JSON.stringify(settings[key]) !== JSON.stringify(originalSettings[key])) {
          changedSettings[key] = settings[key];
        }
      });

      if (Object.keys(changedSettings).length === 0) {
        showMessage('error', 'No changes to save');
        setSaving(false);
        return;
      }

      const res = await fetch(`${API_BASE}/settings/bulk`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ settings: changedSettings })
      });

      if (res.ok) {
        const result = await res.json();
        setVersion(result.version);
        setOriginalSettings(JSON.parse(JSON.stringify(settings)));
        showMessage('success', `${result.updated_keys.length} settings updated`);
      } else {
        showMessage('error', 'Failed to save settings');
      }
    } catch (e) {
      showMessage('error', 'Error saving settings');
    }
    setSaving(false);
  };

  const reloadSettings = async () => {
    if (!confirm('Reload settings from file? This will discard all runtime changes.')) return;

    try {
      const res = await fetch(`${API_BASE}/settings/reload`, { method: 'POST' });
      if (res.ok) {
        await fetchSettings();
        showMessage('success', 'Settings reloaded from file');
      } else {
        showMessage('error', 'Failed to reload settings');
      }
    } catch (e) {
      showMessage('error', 'Error reloading settings');
    }
  };

  const resetChanges = () => {
    if (!confirm('Discard all unsaved changes?')) return;
    setSettings(JSON.parse(JSON.stringify(originalSettings)));
    showMessage('success', 'Changes discarded');
  };

  // Get categories from settings keys
  const categories = ['all', ...new Set(Object.keys(settings).map(key => key.split('.')[0]))];

  // Filter settings
  const filteredSettings = Object.entries(settings)
    .filter(([key]) => {
      if (selectedCategory !== 'all' && !key.startsWith(selectedCategory + '.')) return false;
      if (searchTerm && !key.toLowerCase().includes(searchTerm.toLowerCase())) return false;
      return true;
    })
    .sort(([a], [b]) => a.localeCompare(b));

  // Group by category
  const groupedSettings: Record<string, [string, any][]> = {};
  filteredSettings.forEach(([key, value]) => {
    const category = key.split('.')[0];
    if (!groupedSettings[category]) groupedSettings[category] = [];
    groupedSettings[category].push([key, value]);
  });

  const hasChanges = JSON.stringify(settings) !== JSON.stringify(originalSettings);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-400">Loading settings...</div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-3xl font-bold text-white mb-2">Settings Manager</h1>
          <p className="text-slate-400">
            Centralized runtime configuration • Version: {version}
          </p>
        </div>
        <Link
          href="/admin"
          className="px-4 py-2 text-slate-300 hover:text-white transition-colors"
        >
          ← Back to Dashboard
        </Link>
      </div>

      {/* Message Banner */}
      {message && (
        <div className={`mb-6 p-4 rounded-lg ${
          message.type === 'success'
            ? 'bg-green-600/20 border border-green-500/30 text-green-400'
            : 'bg-red-600/20 border border-red-500/30 text-red-400'
        }`}>
          {message.type === 'success' ? '✓' : '✗'} {message.text}
        </div>
      )}

      {/* Action Bar */}
      <div className="bg-slate-800 rounded-xl p-4 border border-slate-700 mb-6">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-4 flex-1">
            <input
              type="text"
              placeholder="Search settings..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="flex-1 bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-primary"
            />
            <select
              value={selectedCategory}
              onChange={(e) => setSelectedCategory(e.target.value)}
              className="bg-slate-900 border border-slate-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {categories.map(cat => (
                <option key={cat} value={cat}>
                  {cat === 'all' ? 'All Categories' : cat.charAt(0).toUpperCase() + cat.slice(1)}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={reloadSettings}
              className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors flex items-center gap-2"
            >
              <span>🔄</span>
              <span>Reload from File</span>
            </button>

            {hasChanges && (
              <>
                <button
                  onClick={resetChanges}
                  className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors"
                >
                  Reset Changes
                </button>
                <button
                  onClick={saveBulk}
                  disabled={saving}
                  className="px-6 py-2 bg-primary hover:bg-accent text-white rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
                >
                  <span>💾</span>
                  <span>{saving ? 'Saving...' : 'Save All Changes'}</span>
                </button>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Settings Groups */}
      <div className="space-y-6">
        {Object.entries(groupedSettings).map(([category, items]) => (
          <div key={category} className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
            <div className="bg-slate-900/50 px-6 py-4 border-b border-slate-700">
              <h2 className="text-xl font-semibold text-white flex items-center gap-3">
                <span className="text-2xl">{getCategoryIcon(category)}</span>
                <span>{category.charAt(0).toUpperCase() + category.slice(1)}</span>
                <span className="text-sm text-slate-500">({items.length} settings)</span>
              </h2>
            </div>

            <div className="divide-y divide-slate-700">
              {items.map(([key, value]) => (
                <SettingRow
                  key={key}
                  settingKey={key}
                  value={value}
                  originalValue={originalSettings[key]}
                  onUpdate={updateSetting}
                />
              ))}
            </div>
          </div>
        ))}
      </div>

      {filteredSettings.length === 0 && (
        <div className="bg-slate-800 rounded-xl border border-slate-700 p-12 text-center">
          <p className="text-slate-400">No settings found matching your search.</p>
        </div>
      )}
    </div>
  );
}

function SettingRow({ settingKey, value, originalValue, onUpdate }: {
  settingKey: string;
  value: any;
  originalValue: any;
  onUpdate: (key: string, value: any) => void;
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(value);
  const isModified = JSON.stringify(value) !== JSON.stringify(originalValue);

  const valueType = Array.isArray(value) ? 'array'
    : typeof value === 'object' && value !== null ? 'object'
    : typeof value;

  const handleSave = () => {
    onUpdate(settingKey, editValue);
    setIsEditing(false);
  };

  const handleCancel = () => {
    setEditValue(value);
    setIsEditing(false);
  };

  return (
    <div className={`p-4 hover:bg-slate-700/30 transition-colors ${isModified ? 'bg-yellow-500/5' : ''}`}>
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <code className="text-sm font-mono text-primary break-all">
              {settingKey}
            </code>
            {isModified && (
              <span className="px-2 py-0.5 bg-yellow-500/20 text-yellow-400 text-xs rounded-full border border-yellow-500/30">
                Modified
              </span>
            )}
            <span className="px-2 py-0.5 bg-slate-700 text-slate-400 text-xs rounded">
              {valueType}
            </span>
          </div>

          {!isEditing ? (
            <div className="mt-2">
              <ValueDisplay value={value} type={valueType} />
            </div>
          ) : (
            <div className="mt-2">
              <ValueEditor
                value={editValue}
                type={valueType}
                onChange={setEditValue}
              />
            </div>
          )}
        </div>

        <div className="flex items-center gap-2">
          {!isEditing ? (
            <button
              onClick={() => setIsEditing(true)}
              className="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-white text-sm rounded transition-colors"
            >
              Edit
            </button>
          ) : (
            <>
              <button
                onClick={handleCancel}
                className="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-white text-sm rounded transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                className="px-3 py-1.5 bg-primary hover:bg-accent text-white text-sm rounded transition-colors"
              >
                Save
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function ValueDisplay({ value, type }: { value: any; type: string }) {
  if (type === 'boolean') {
    return (
      <div className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm ${
        value ? 'bg-green-500/20 text-green-400 border border-green-500/30' : 'bg-slate-700/50 text-slate-400 border border-slate-600'
      }`}>
        <div className={`w-2 h-2 rounded-full ${value ? 'bg-green-400' : 'bg-slate-500'}`} />
        {value ? 'Enabled' : 'Disabled'}
      </div>
    );
  }

  if (type === 'array') {
    return (
      <div className="space-y-1">
        {(value as any[]).map((item, idx) => (
          <div key={idx} className="text-slate-300 text-sm font-mono bg-slate-900/50 px-3 py-1.5 rounded border border-slate-700">
            {JSON.stringify(item)}
          </div>
        ))}
      </div>
    );
  }

  if (type === 'object') {
    return (
      <pre className="text-slate-300 text-sm font-mono bg-slate-900/50 px-3 py-2 rounded border border-slate-700 overflow-x-auto">
        {JSON.stringify(value, null, 2)}
      </pre>
    );
  }

  return (
    <div className="text-slate-300 text-sm font-mono bg-slate-900/50 px-3 py-1.5 rounded border border-slate-700">
      {String(value)}
    </div>
  );
}

function ValueEditor({ value, type, onChange }: { value: any; type: string; onChange: (v: any) => void }) {
  if (type === 'boolean') {
    return (
      <button
        onClick={() => onChange(!value)}
        className={`relative inline-flex h-7 w-14 items-center rounded-full transition-colors ${
          value ? 'bg-primary' : 'bg-slate-600'
        }`}
      >
        <span className={`inline-block h-5 w-5 transform rounded-full bg-white transition-transform ${
          value ? 'translate-x-8' : 'translate-x-1'
        }`} />
      </button>
    );
  }

  if (type === 'number') {
    return (
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(parseFloat(e.target.value))}
        className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:ring-2 focus:ring-primary"
      />
    );
  }

  if (type === 'array' || type === 'object') {
    return (
      <textarea
        value={JSON.stringify(value, null, 2)}
        onChange={(e) => {
          try {
            onChange(JSON.parse(e.target.value));
          } catch {}
        }}
        rows={Math.min(10, JSON.stringify(value, null, 2).split('\n').length)}
        className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-primary"
      />
    );
  }

  return (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-white focus:outline-none focus:ring-2 focus:ring-primary"
    />
  );
}

function getCategoryIcon(category: string): string {
  const icons: Record<string, string> = {
    app: '🚀',
    server: '🖥️',
    infrastructure: '🗄️',
    auth: '🔐',
    inference: '🤖',
    models: '🧠',
    search: '🔍',
    ast: '🌳',
    mcp: '🔌',
    model_management: '⚡',
    rag: '💬',
    indexing: '📑',
    worker: '⚙️',
    logging: '📝',
    telemetry: '📊',
    features: '✨',
    defaults: '📋',
    performance: '⚡',
    admin: '👑',
    metrics: '📈',
    cli: '💻',
    messages: '💬',
  };
  return icons[category] || '⚙️';
}
