import { motion } from 'framer-motion';
import {
  PlusIcon,
  PlayIcon,
  ArrowPathIcon,
  DocumentArrowUpIcon,
  GlobeAltIcon,
} from '@heroicons/react/24/outline';
import ProxyTable from '@/components/proxies/ProxyTable';
import ProxyFormModal from '@/components/proxies/ProxyFormModal';
import BatchImportModal from '@/components/proxies/BatchImportModal';
import { useProxies } from '@/hooks/useProxies';

function ProxiesPage() {
  const {
    fileInputRef,
    proxies,
    loading,
    showModal,
    showBatchModal,
    editingProxy,
    formData,
    setFormData,
    saving,
    batchInput,
    setBatchInput,
    batchImporting,
    testingId,
    testingAll,
    testResults,
    deleteConfirmId,
    setDeleteConfirmId,
    deleting,
    openCreateModal,
    openEditModal,
    openBatchModal,
    closeModal,
    closeBatchModal,
    handleSubmit,
    handleBatchImport,
    handleTestProxy,
    handleTestAllProxies,
    handleConfirmDelete,
    handleToggle,
    handleFileUpload,
  } = useProxies();

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-apple-blue" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <input type="file" ref={fileInputRef} onChange={handleFileUpload} accept=".txt,.csv,.conf" className="hidden" />

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Proxies</h1>
          <p className="text-apple-gray-500 mt-1">Manage proxy nodes for API requests</p>
        </div>
        <div className="flex items-center gap-3">
          {proxies.length > 0 && (
            <button onClick={handleTestAllProxies} className="btn btn-secondary" disabled={testingAll}>
              {testingAll ? <ArrowPathIcon className="w-5 h-5 mr-2 animate-spin" /> : <PlayIcon className="w-5 h-5 mr-2" />}
              Test All
            </button>
          )}
          <div className="relative group">
            <button onClick={openBatchModal} className="btn btn-secondary">
              <DocumentArrowUpIcon className="w-5 h-5 mr-2" /> Import
            </button>
          </div>
          <button onClick={openCreateModal} className="btn btn-primary">
            <PlusIcon className="w-5 h-5 mr-2" /> Add Proxy
          </button>
        </div>
      </div>

      <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="card overflow-x-auto">
        {proxies.length === 0 ? (
          <div className="text-center py-16">
            <div className="w-16 h-16 bg-blue-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
              <GlobeAltIcon className="w-8 h-8 text-apple-blue" />
            </div>
            <h3 className="text-lg font-semibold text-apple-gray-900 mb-1">No Proxies Configured</h3>
            <p className="text-apple-gray-500 text-sm mb-6 max-w-sm mx-auto">
              Add proxy nodes to distribute and load-balance your LLM API requests.
            </p>
            <button onClick={openCreateModal} className="btn btn-primary rounded-xl">
              <PlusIcon className="w-5 h-5 mr-2" /> Add your first proxy
            </button>
          </div>
        ) : (
          <ProxyTable
            proxies={proxies}
            testResults={testResults}
            testingId={testingId}
            deleteConfirmId={deleteConfirmId}
            deleting={deleting}
            onTest={handleTestProxy}
            onEdit={openEditModal}
            onToggle={handleToggle}
            onDeleteClick={setDeleteConfirmId}
            onConfirmDelete={handleConfirmDelete}
            onCancelDelete={() => setDeleteConfirmId(null)}
          />
        )}
      </motion.div>

      <ProxyFormModal
        isOpen={showModal}
        editingProxy={editingProxy}
        formData={formData}
        proxies={proxies}
        saving={saving}
        onFormChange={setFormData}
        onSubmit={handleSubmit}
        onClose={closeModal}
      />

      <BatchImportModal
        isOpen={showBatchModal}
        batchInput={batchInput}
        importing={batchImporting}
        onInputChange={setBatchInput}
        onImport={handleBatchImport}
        onClose={closeBatchModal}
      />
    </div>
  );
}

export default ProxiesPage;
