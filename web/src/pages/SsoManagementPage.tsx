import { useState, useMemo } from 'react';
import { useQuery, useMutation } from '@apollo/client/react';
import { gql } from '@apollo/client';
import { motion } from 'framer-motion';
import toast from 'react-hot-toast';
import {
  PlusIcon,
  TrashIcon,
  PencilIcon,
  ShieldCheckIcon,
  KeyIcon,
  GlobeAltIcon,
} from '@heroicons/react/24/outline';
import {
  IDENTITY_PROVIDERS_QUERY,
  CREATE_IDENTITY_PROVIDER,
  UPDATE_IDENTITY_PROVIDER,
  DELETE_IDENTITY_PROVIDER,
} from '@/lib/graphql/operations/sso';

/* eslint-disable @typescript-eslint/no-explicit-any */

const MY_ORGS_INLINE = gql`query MyOrgsForSSO { myOrganizations { id name } }`;

function SsoManagementPage() {
  // Load user's orgs to scope IdP listing
  const { data: orgResult } = useQuery<any>(MY_ORGS_INLINE);
  const orgs = useMemo(() => orgResult?.myOrganizations || [], [orgResult]);
  const [selectedOrgId, setSelectedOrgId] = useState('');

  useMemo(() => {
    if (orgs.length > 0 && !selectedOrgId) setSelectedOrgId(orgs[0].id);
  }, [orgs, selectedOrgId]);

  const { data: idpData, loading, refetch } = useQuery<any>(IDENTITY_PROVIDERS_QUERY, {
    variables: { orgId: selectedOrgId },
    skip: !selectedOrgId,
  });
  const idps = useMemo(() => idpData?.identityProviders || [], [idpData]);

  const [createMut] = useMutation(CREATE_IDENTITY_PROVIDER);
  const [updateMut] = useMutation(UPDATE_IDENTITY_PROVIDER);
  const [deleteMut] = useMutation(DELETE_IDENTITY_PROVIDER);

  // Modal state
  const [showModal, setShowModal] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [saving, setSaving] = useState(false);

  // Form fields
  const [idpType, setIdpType] = useState<'OIDC' | 'SAML'>('OIDC');
  const [name, setName] = useState('');
  const [domains, setDomains] = useState('');
  const [oidcClientId, setOidcClientId] = useState('');
  const [oidcClientSecret, setOidcClientSecret] = useState('');
  const [oidcIssuerUrl, setOidcIssuerUrl] = useState('');
  const [samlEntityId, setSamlEntityId] = useState('');
  const [samlSsoUrl, setSamlSsoUrl] = useState('');
  const [samlIdpCert, setSamlIdpCert] = useState('');
  const [enableJit, setEnableJit] = useState(true);
  const [defaultRole, setDefaultRole] = useState('MEMBER');

  const resetForm = () => {
    setEditing(null);
    setIdpType('OIDC');
    setName('');
    setDomains('');
    setOidcClientId('');
    setOidcClientSecret('');
    setOidcIssuerUrl('');
    setSamlEntityId('');
    setSamlSsoUrl('');
    setSamlIdpCert('');
    setEnableJit(true);
    setDefaultRole('MEMBER');
  };

  const openCreate = () => { resetForm(); setShowModal(true); };
  const openEdit = (idp: any) => {
    setEditing(idp);
    setIdpType(idp.type);
    setName(idp.name);
    setDomains(idp.domains);
    setOidcClientId(idp.oidcClientId || '');
    setOidcIssuerUrl(idp.oidcIssuerUrl || '');
    setSamlEntityId(idp.samlEntityId || '');
    setSamlSsoUrl(idp.samlSsoUrl || '');
    setEnableJit(idp.enableJit);
    setDefaultRole(idp.defaultRole);
    setShowModal(true);
  };

  const handleSave = async () => {
    if (!name.trim() || !domains.trim()) {
      toast.error('Name and domains are required');
      return;
    }
    setSaving(true);
    try {
      if (editing) {
        await updateMut({
          variables: {
            id: editing.id,
            input: {
              name, domains, enableJit, defaultRole,
              ...(idpType === 'OIDC'
                ? { oidcClientId, oidcClientSecret: oidcClientSecret || undefined, oidcIssuerUrl }
                : { samlEntityId, samlSsoUrl, samlIdpCert: samlIdpCert || undefined }),
            },
          },
        });
        toast.success('Identity provider updated');
      } else {
        await createMut({
          variables: {
            input: {
              orgId: selectedOrgId, type: idpType, name, domains, enableJit, defaultRole,
              ...(idpType === 'OIDC'
                ? { oidcClientId, oidcClientSecret, oidcIssuerUrl }
                : { samlEntityId, samlSsoUrl, samlIdpCert }),
            },
          },
        });
        toast.success('Identity provider created');
      }
      setShowModal(false);
      refetch();
    } catch (e: any) {
      toast.error(e.message || 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm('Delete this identity provider? SSO users will no longer be able to log in.')) return;
    try {
      await deleteMut({ variables: { id } });
      toast.success('Identity provider deleted');
      refetch();
    } catch {
      toast.error('Failed to delete');
    }
  };

  const handleToggle = async (idp: any) => {
    try {
      await updateMut({ variables: { id: idp.id, input: { isActive: !idp.isActive } } });
      toast.success(`Provider ${idp.isActive ? 'disabled' : 'enabled'}`);
      refetch();
    } catch {
      toast.error('Failed to toggle status');
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">SSO / Identity Providers</h1>
          <p className="text-apple-gray-500 mt-1">Configure OIDC and SAML identity providers for your organization</p>
        </div>
        <button onClick={openCreate} className="btn btn-primary" disabled={!selectedOrgId}>
          <PlusIcon className="w-5 h-5 mr-2" /> Add Provider
        </button>
      </div>

      {orgs.length > 1 && (
        <div className="flex items-center gap-3">
          <label className="text-sm font-medium text-apple-gray-700">Organization:</label>
          <select value={selectedOrgId} onChange={(e) => setSelectedOrgId(e.target.value)} className="input max-w-xs">
            {orgs.map((o: any) => <option key={o.id} value={o.id}>{o.name}</option>)}
          </select>
        </div>
      )}

      {idps.length === 0 ? (
        <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="card text-center py-16">
          <ShieldCheckIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-apple-gray-900 mb-1">No Identity Providers</h3>
          <p className="text-apple-gray-500 text-sm mb-6 max-w-sm mx-auto">
            Add an OIDC or SAML provider to enable single sign-on for your organization.
          </p>
          <button onClick={openCreate} className="btn btn-primary rounded-xl" disabled={!selectedOrgId}>
            Configure SSO
          </button>
        </motion.div>
      ) : (
        <div className="grid gap-4">
          {idps.map((idp: any, i: number) => (
            <motion.div
              key={idp.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: i * 0.05 }}
              className="card p-5"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className={`p-3 rounded-xl ${idp.type === 'OIDC' ? 'bg-blue-50 text-apple-blue' : 'bg-purple-50 text-purple-600'}`}>
                    {idp.type === 'OIDC' ? <KeyIcon className="w-6 h-6" /> : <GlobeAltIcon className="w-6 h-6" />}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold text-apple-gray-900">{idp.name}</h3>
                      <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                        idp.type === 'OIDC' ? 'bg-blue-100 text-blue-700' : 'bg-purple-100 text-purple-700'
                      }`}>
                        {idp.type}
                      </span>
                    </div>
                    <p className="text-sm text-apple-gray-500 mt-0.5">
                      Domains: <code className="text-xs bg-apple-gray-100 px-1.5 py-0.5 rounded">{idp.domains}</code>
                    </p>
                    {idp.type === 'OIDC' && idp.oidcIssuerUrl && (
                      <p className="text-xs text-apple-gray-400 mt-1">Issuer: {idp.oidcIssuerUrl}</p>
                    )}
                    {idp.type === 'SAML' && idp.samlSsoUrl && (
                      <p className="text-xs text-apple-gray-400 mt-1">SSO URL: {idp.samlSsoUrl}</p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-3">
                  <div className="flex items-center gap-2 text-xs text-apple-gray-500">
                    <span>JIT: {idp.enableJit ? 'Yes' : 'No'}</span>
                    <span>Role: {idp.defaultRole}</span>
                  </div>
                  <button
                    onClick={() => handleToggle(idp)}
                    className={`relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
                      idp.isActive ? 'bg-apple-green' : 'bg-apple-gray-200'
                    }`}
                  >
                    <span className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow transition ${
                      idp.isActive ? 'translate-x-4' : 'translate-x-0'
                    }`} />
                  </button>
                  <button onClick={() => openEdit(idp)} className="p-1.5 text-apple-gray-400 hover:text-apple-blue transition-colors" title="Edit">
                    <PencilIcon className="w-4 h-4" />
                  </button>
                  <button onClick={() => handleDelete(idp.id)} className="p-1.5 text-apple-gray-400 hover:text-red-500 transition-colors" title="Delete">
                    <TrashIcon className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      )}

      {/* Create/Edit Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="bg-[var(--theme-bg-card)] rounded-apple-lg shadow-apple-xl p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto"
          >
            <h2 className="text-xl font-semibold text-apple-gray-900 mb-4">
              {editing ? 'Edit Identity Provider' : 'Add Identity Provider'}
            </h2>

            <div className="space-y-4">
              {!editing && (
                <div>
                  <label className="label">Protocol</label>
                  <div className="grid grid-cols-2 gap-3 mt-1">
                    {(['OIDC', 'SAML'] as const).map((t) => (
                      <button
                        key={t}
                        type="button"
                        onClick={() => setIdpType(t)}
                        className={`p-3 rounded-xl border text-center text-sm font-medium transition-colors ${
                          idpType === t ? 'border-apple-blue bg-apple-blue/5 text-apple-blue' : 'border-apple-gray-200 text-apple-gray-600 hover:border-apple-gray-300'
                        }`}
                      >
                        {t === 'OIDC' ? 'OpenID Connect' : 'SAML 2.0'}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <label className="label">Display Name *</label>
                <input type="text" value={name} onChange={(e) => setName(e.target.value)} className="input" placeholder="e.g. Google Workspace" />
              </div>

              <div>
                <label className="label">Allowed Domains *</label>
                <input type="text" value={domains} onChange={(e) => setDomains(e.target.value)} className="input" placeholder="company.com,subsidiary.com" />
                <p className="text-xs text-apple-gray-400 mt-1">Comma-separated email domains</p>
              </div>

              {idpType === 'OIDC' ? (
                <>
                  <div>
                    <label className="label">Client ID</label>
                    <input type="text" value={oidcClientId} onChange={(e) => setOidcClientId(e.target.value)} className="input font-mono text-sm" />
                  </div>
                  <div>
                    <label className="label">Client Secret</label>
                    <input type="password" value={oidcClientSecret} onChange={(e) => setOidcClientSecret(e.target.value)} className="input" placeholder={editing ? '(unchanged)' : ''} />
                  </div>
                  <div>
                    <label className="label">Issuer URL</label>
                    <input type="url" value={oidcIssuerUrl} onChange={(e) => setOidcIssuerUrl(e.target.value)} className="input font-mono text-sm" placeholder="https://accounts.google.com" />
                  </div>
                </>
              ) : (
                <>
                  <div>
                    <label className="label">Entity ID</label>
                    <input type="text" value={samlEntityId} onChange={(e) => setSamlEntityId(e.target.value)} className="input font-mono text-sm" />
                  </div>
                  <div>
                    <label className="label">SSO URL</label>
                    <input type="url" value={samlSsoUrl} onChange={(e) => setSamlSsoUrl(e.target.value)} className="input font-mono text-sm" />
                  </div>
                  <div>
                    <label className="label">IdP Certificate (PEM)</label>
                    <textarea value={samlIdpCert} onChange={(e) => setSamlIdpCert(e.target.value)} className="input font-mono text-xs h-24" placeholder={editing ? '(unchanged)' : '-----BEGIN CERTIFICATE-----'} />
                  </div>
                </>
              )}

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="label">Default Role</label>
                  <select value={defaultRole} onChange={(e) => setDefaultRole(e.target.value)} className="input">
                    <option value="MEMBER">Member</option>
                    <option value="ADMIN">Admin</option>
                    <option value="READONLY">Read Only</option>
                  </select>
                </div>
                <div className="flex flex-col justify-end pb-1">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" checked={enableJit} onChange={(e) => setEnableJit(e.target.checked)} className="rounded border-apple-gray-300 text-apple-blue" />
                    <span className="text-sm text-apple-gray-700">Just-in-Time provisioning</span>
                  </label>
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-3 mt-8">
              <button onClick={() => setShowModal(false)} className="btn btn-secondary">Cancel</button>
              <button onClick={handleSave} className="btn btn-primary" disabled={saving}>
                {saving ? 'Saving...' : 'Save Provider'}
              </button>
            </div>
          </motion.div>
        </div>
      )}
    </div>
  );
}

export default SsoManagementPage;
