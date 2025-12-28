'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { listStores, createStore, deleteStore } from '@/lib/api';
import { ErrorBanner, LoadingSpinner } from '@/components';
import type { StoreInfo } from '@/types';

export default function AdminPage() {
  const [stores, setStores] = useState<StoreInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newStoreName, setNewStoreName] = useState('');
  const [newStoreDescription, setNewStoreDescription] = useState('');
  const [creating, setCreating] = useState(false);

  const fetchStores = async () => {
    setLoading(true);
    setError(null);
    try {
      const storeList = await listStores();
      setStores(storeList);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch stores');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStores();
  }, []);

  const handleCreateStore = async () => {
    if (!newStoreName.trim()) return;
    setCreating(true);
    setError(null);
    try {
      await createStore(newStoreName.trim(), newStoreDescription.trim() || undefined);
      setNewStoreName('');
      setNewStoreDescription('');
      await fetchStores();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create store');
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteStore = async (storeName: string) => {
    if (!confirm(`Are you sure you want to delete store "${storeName}"? This cannot be undone.`)) {
      return;
    }
    setError(null);
    try {
      await deleteStore(storeName);
      await fetchStores();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete store');
    }
  };

  const formatDate = (dateStr: string): string => {
    if (!dateStr) return 'Never';
    const date = new Date(dateStr);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
  };

  return (
    <main className="main admin-page">
      <div className="admin-header">
        <h1>🗄️ Store Management</h1>
        <p className="admin-subtitle">Manage your search indexes and view statistics</p>
      </div>

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      {/* Create Store Form */}
      <div className="admin-card">
        <h2 className="admin-card-title">➕ Create New Store</h2>
        <div className="create-store-form">
          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Store Name</label>
              <input
                type="text"
                className="form-input"
                placeholder="my-project"
                value={newStoreName}
                onChange={(e) => setNewStoreName(e.target.value)}
              />
            </div>
            <div className="form-group">
              <label className="form-label">Description (optional)</label>
              <input
                type="text"
                className="form-input"
                placeholder="My project codebase"
                value={newStoreDescription}
                onChange={(e) => setNewStoreDescription(e.target.value)}
              />
            </div>
            <button
              type="button"
              className="btn-primary"
              onClick={handleCreateStore}
              disabled={creating || !newStoreName.trim()}
            >
              {creating ? 'Creating...' : 'Create Store'}
            </button>
          </div>
        </div>
      </div>

      {/* Stores List */}
      <div className="admin-card">
        <h2 className="admin-card-title">📚 Stores</h2>
        {loading ? (
          <LoadingSpinner message="Loading stores..." />
        ) : stores.length === 0 ? (
          <div className="empty-state">
            <span className="empty-icon">📭</span>
            <p>No stores yet. Create one to get started!</p>
          </div>
        ) : (
          <div className="stores-table-wrapper">
            <table className="stores-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Description</th>
                  <th>Documents</th>
                  <th>Created</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {stores.map((store) => (
                  <tr key={store.name}>
                    <td>
                      <Link href={`/admin/stores/${store.name}`} className="store-link">
                        {store.name}
                      </Link>
                    </td>
                    <td className="text-muted">{store.description || '-'}</td>
                    <td>{store.doc_count ?? '-'}</td>
                    <td className="text-muted">{formatDate(store.created_at)}</td>
                    <td>
                      <div className="action-buttons">
                        <Link href={`/admin/stores/${store.name}`} className="btn-icon" title="View details">
                          👁️
                        </Link>
                        {store.name !== 'default' && (
                          <button
                            className="btn-icon btn-danger"
                            onClick={() => handleDeleteStore(store.name)}
                            title="Delete store"
                          >
                            🗑️
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </main>
  );
}
