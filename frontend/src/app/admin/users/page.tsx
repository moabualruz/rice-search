'use client';

const users = [
  { id: '1', email: 'admin@rice.local', role: 'admin', org: 'default', lastSeen: '2024-12-31' },
  { id: '2', email: 'user@rice.local', role: 'member', org: 'default', lastSeen: '2024-12-30' },
];

const roles = ['admin', 'member', 'readonly'];

export default function UsersPage() {
  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white">User Management</h1>
          <p className="text-slate-400">Manage users and permissions</p>
        </div>
        <button className="px-6 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700">
          + Add User
        </button>
      </div>

      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <table className="w-full">
          <thead className="bg-slate-900">
            <tr>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Email</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Role</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Organization</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Last Seen</th>
              <th className="px-6 py-4 text-left text-slate-400 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700">
            {users.map((user) => (
              <tr key={user.id} className="hover:bg-slate-700/30">
                <td className="px-6 py-4 text-white">{user.email}</td>
                <td className="px-6 py-4">
                  <select 
                    defaultValue={user.role}
                    className="px-3 py-1 bg-slate-900 border border-slate-600 rounded text-white"
                  >
                    {roles.map(role => (
                      <option key={role} value={role}>{role}</option>
                    ))}
                  </select>
                </td>
                <td className="px-6 py-4 text-slate-300">{user.org}</td>
                <td className="px-6 py-4 text-slate-400">{user.lastSeen}</td>
                <td className="px-6 py-4">
                  <button className="text-red-400 hover:text-red-300">Remove</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="mt-8 p-6 bg-slate-800/50 rounded-xl border border-dashed border-slate-600 text-center">
        <p className="text-slate-400">
          Full user management requires Keycloak integration.<br/>
          Users are currently managed through Keycloak admin console.
        </p>
      </div>
    </div>
  );
}
