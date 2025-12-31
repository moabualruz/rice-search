'use client';

import { useState, useEffect } from 'react';

interface User {
  id: string;
  email: string;
  role: string;
  org_id: string;
  active: boolean;
  created_at: string;
}

const API_BASE = 'http://localhost:8000/api/v1/admin/public';
const roles = ['admin', 'member', 'readonly'];

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState('member');

  const fetchUsers = async () => {
    try {
      const res = await fetch(`${API_BASE}/users`);
      if (res.ok) {
        const data = await res.json();
        setUsers(data.users || []);
      }
    } catch (e) {
      console.error('Failed to fetch users', e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const updateRole = async (user: User, newRole: string) => {
    try {
      const res = await fetch(`${API_BASE}/users/${user.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ role: newRole })
      });
      if (res.ok) {
        const data = await res.json();
        setUsers(users.map(u => u.id === user.id ? data.user : u));
        showMessage('success', `Role updated to ${newRole}`);
      }
    } catch (e) {
      showMessage('error', 'Failed to update role');
    }
  };

  const toggleActive = async (user: User) => {
    try {
      const res = await fetch(`${API_BASE}/users/${user.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ active: !user.active })
      });
      if (res.ok) {
        const data = await res.json();
        setUsers(users.map(u => u.id === user.id ? data.user : u));
        showMessage('success', `User ${user.active ? 'deactivated' : 'activated'}`);
      }
    } catch (e) {
      showMessage('error', 'Failed to update user');
    }
  };

  const addUser = async () => {
    if (!newEmail.trim()) return;
    try {
      const res = await fetch(`${API_BASE}/users`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: newEmail, role: newRole })
      });
      if (res.ok) {
        const data = await res.json();
        setUsers([...users, data.user]);
        setNewEmail('');
        showMessage('success', `User ${newEmail} created`);
      }
    } catch (e) {
      showMessage('error', 'Failed to create user');
    }
  };

  const deleteUser = async (user: User) => {
    if (user.id === 'admin-1') {
      showMessage('error', 'Cannot delete primary admin');
      return;
    }
    try {
      const res = await fetch(`${API_BASE}/users/${user.id}`, { method: 'DELETE' });
      if (res.ok) {
        setUsers(users.filter(u => u.id !== user.id));
        showMessage('success', `User ${user.email} deleted`);
      }
    } catch (e) {
      showMessage('error', 'Failed to delete user');
    }
  };

  if (loading) return <div className="text-slate-400">Loading...</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white">User Management</h1>
          <p className="text-slate-400">Manage users and permissions</p>
        </div>
      </div>

      {message && (
        <div className={`mb-6 p-4 rounded-lg ${
          message.type === 'success' 
            ? 'bg-green-600/20 border border-green-500/30 text-green-400'
            : 'bg-red-600/20 border border-red-500/30 text-red-400'
        }`}>
          {message.type === 'success' ? '✓' : '✗'} {message.text}
        </div>
      )}

      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-900">
            <tr>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Email</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Role</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Organization</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Status</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {users.map((user) => (
              <tr key={user.id} className="hover:bg-slate-700/30">
                <td className="px-6 py-4 text-white">{user.email}</td>
                <td className="px-6 py-4">
                  <select 
                    value={user.role}
                    onChange={(e) => updateRole(user, e.target.value)}
                    className="px-3 py-1 bg-slate-900 border border-slate-600 rounded text-white"
                  >
                    {roles.map(role => (
                      <option key={role} value={role}>{role}</option>
                    ))}
                  </select>
                </td>
                <td className="px-6 py-4 text-slate-300">{user.org_id}</td>
                <td className="px-6 py-4">
                  <button
                    onClick={() => toggleActive(user)}
                    className={`px-3 py-1 rounded-lg text-sm ${
                      user.active 
                        ? 'bg-green-500/20 text-green-400' 
                        : 'bg-red-500/20 text-red-400'
                    }`}
                  >
                    {user.active ? 'Active' : 'Inactive'}
                  </button>
                </td>
                <td className="px-6 py-4">
                  <button 
                    onClick={() => deleteUser(user)}
                    className="text-red-400 hover:text-red-300"
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Add New User */}
      <div className="mt-8 bg-slate-800 rounded-xl p-6 border border-slate-700">
        <h3 className="text-lg font-semibold text-white mb-4">Add New User</h3>
        <div className="grid grid-cols-3 gap-4">
          <input
            type="email"
            value={newEmail}
            onChange={(e) => setNewEmail(e.target.value)}
            placeholder="Email address"
            className="px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white col-span-2"
          />
          <select 
            value={newRole}
            onChange={(e) => setNewRole(e.target.value)}
            className="px-4 py-2 bg-slate-900 border border-slate-600 rounded-lg text-white"
          >
            {roles.map(role => (
              <option key={role} value={role}>{role}</option>
            ))}
          </select>
        </div>
        <button 
          onClick={addUser}
          className="mt-4 px-6 py-2 bg-primary text-white rounded-lg hover:bg-accent"
        >
          Add User
        </button>
      </div>
    </div>
  );
}
