/* eslint-disable @typescript-eslint/no-explicit-any */
import { ShieldCheckIcon, HandRaisedIcon, XMarkIcon, CheckCircleIcon, BeakerIcon, SparklesIcon } from '@heroicons/react/24/outline';
import { POLICY_PRESETS, PII_RULES, useDlpSettings } from '@/components/dlp-settings';
import { useTranslation } from '@/lib/i18n';

export default function DlpSettingsPage() {
  const { t } = useTranslation();
  const {
    saving, isAdmin,
    orgs, selectedOrgId, setSelectedOrgId,
    projects, currentProjectId, setCurrentProjectId,
    loading, config, activePresetId, isEnabled,
    testInput, setTestInput, testResult, testing,
    customRegexInput, setCustomRegexInput,
    applyPreset, handleToggleEnable, handleUpdateStrategy, handleToggleMask,
    handleAddCustomRegex, handleRemoveCustomRegex, handleRunSandbox, handlePublishToAllProjects,
  } = useDlpSettings();

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--theme-color-primary)]" />
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto space-y-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-[var(--theme-text-primary)] border-none m-0 p-0">{t('dlp.title')}</h1>
          <p className="text-sm text-[var(--theme-text-tertiary)] mt-1 max-w-xl">
            {t('dlp.description')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <select title={t('common.organization')} value={selectedOrgId} onChange={(e) => setSelectedOrgId(e.target.value)}
            className="block w-40 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm px-3 py-2 text-sm focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)]">
            <option value="" disabled>{t('common.select_org')}</option>
            {orgs.map((o) => <option key={o.id} value={o.id}>{o.name}</option>)}
          </select>
          <select title={t('common.project')} value={currentProjectId} onChange={(e) => setCurrentProjectId(e.target.value)}
            className="block w-40 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm px-3 py-2 text-sm focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)]">
            <option value="" disabled>{t('common.select_project')}</option>
            {projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </div>
      </div>

      {!currentProjectId ? (
        <div className="p-12 text-center text-[var(--theme-text-tertiary)] card">
          <ShieldCheckIcon className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p>{t('dlp.select_project_hint')}</p>
        </div>
      ) : (
      <>
        {/* Global Toggle */}
        <div className="card p-6 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className={`p-3 rounded-2xl flex items-center justify-center transition-colors ${isEnabled ? 'bg-green-500/15 text-green-500' : 'bg-[var(--theme-bg-subtle)] text-[var(--theme-text-tertiary)]'}`}>
              <ShieldCheckIcon className="w-6 h-6" />
            </div>
            <div>
              <h3 className="text-lg font-semibold text-[var(--theme-text-primary)]">{t('dlp.protection_status')}</h3>
              <p className="text-sm text-[var(--theme-text-secondary)]">
                {isEnabled
                  ? <>{t('dlp.status_active_prefix')}<strong className="text-[var(--theme-text-primary)]">{activePresetId === 'none' ? t('common.disabled') : activePresetId.charAt(0).toUpperCase() + activePresetId.slice(1)}</strong>{t('dlp.status_active_suffix')}</>
                  : t('dlp.status_disabled')}
              </p>
            </div>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" className="sr-only peer" checked={isEnabled} onChange={(e) => handleToggleEnable(e.target.checked)} disabled={saving} />
            <div className="w-11 h-6 bg-[var(--theme-bg-subtle)] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-green-500"></div>
          </label>
        </div>

        {/* Policy Presets */}
        <div>
          <h2 className="text-lg font-semibold text-[var(--theme-text-primary)] mb-1">{t('dlp.quick_presets')}</h2>
          <p className="text-sm text-[var(--theme-text-tertiary)] mb-4">{t('dlp.quick_presets_desc')}</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {POLICY_PRESETS.map(preset => {
              const isActive = activePresetId === preset.id;
              return (
                <button key={preset.id} onClick={() => applyPreset(preset)} disabled={saving}
                  className={`relative flex flex-col items-start p-5 rounded-2xl border-2 text-left transition-all duration-200 hover:shadow-md disabled:opacity-50 ${isActive ? `${preset.borderColor} ${preset.bgColor} shadow-sm` : 'border-[var(--theme-border-default)] hover:border-[var(--theme-border-hover)] bg-[var(--theme-bg-surface)]'}`}>
                  {isActive && <CheckCircleIcon className={`absolute top-3 right-3 w-5 h-5 ${preset.color}`} />}
                  <div className={`p-2.5 rounded-xl mb-3 ${isActive ? preset.bgColor : 'bg-[var(--theme-bg-subtle)]'}`}>
                    <span className={isActive ? preset.color : 'text-[var(--theme-text-tertiary)]'}>{preset.icon}</span>
                  </div>
                  <span className={`text-base font-semibold mb-1 ${isActive ? preset.color : 'text-[var(--theme-text-primary)]'}`}>{preset.name}</span>
                  <p className="text-xs text-[var(--theme-text-tertiary)] leading-relaxed">{preset.description}</p>
                </button>
              );
            })}
          </div>
        </div>

        {/* Main Content Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left Column: Settings */}
          <div className="lg:col-span-2 space-y-6">
            {/* Strategy Selection */}
            <div className={`card p-6 transition-opacity ${!isEnabled ? 'opacity-40 pointer-events-none' : ''}`}>
              <h3 className="text-base font-semibold text-[var(--theme-text-primary)] mb-1">{t('dlp.interception_strategy')}</h3>
              <p className="text-sm text-[var(--theme-text-tertiary)] mb-4">{t('dlp.interception_strategy_desc')}</p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <button onClick={() => handleUpdateStrategy('REDACT')}
                  className={`flex items-start gap-3 p-4 rounded-xl border-2 text-left transition-all ${config?.strategy === 'REDACT' ? 'border-green-500 bg-green-500/10' : 'border-[var(--theme-border-default)] hover:border-[var(--theme-border-hover)]'}`}>
                  <ShieldCheckIcon className={`w-5 h-5 mt-0.5 flex-shrink-0 ${config?.strategy === 'REDACT' ? 'text-green-500' : 'text-[var(--theme-text-tertiary)]'}`} />
                  <div>
                    <span className="font-semibold text-[var(--theme-text-primary)] block">{t('dlp.strategy_redact')}</span>
                    <span className="text-xs text-[var(--theme-text-tertiary)]">{t('dlp.strategy_redact_desc')}</span>
                  </div>
                </button>
                <button onClick={() => handleUpdateStrategy('BLOCK')}
                  className={`flex items-start gap-3 p-4 rounded-xl border-2 text-left transition-all ${config?.strategy === 'BLOCK' ? 'border-red-500 bg-red-500/10' : 'border-[var(--theme-border-default)] hover:border-[var(--theme-border-hover)]'}`}>
                  <HandRaisedIcon className={`w-5 h-5 mt-0.5 flex-shrink-0 ${config?.strategy === 'BLOCK' ? 'text-red-500' : 'text-[var(--theme-text-tertiary)]'}`} />
                  <div>
                    <span className="font-semibold text-[var(--theme-text-primary)] block">{t('dlp.strategy_block')}</span>
                    <span className="text-xs text-[var(--theme-text-tertiary)]">{t('dlp.strategy_block_desc')}</span>
                  </div>
                </button>
              </div>
            </div>

            {/* PII Rules */}
            <div className={`card overflow-hidden transition-opacity ${!isEnabled ? 'opacity-40 pointer-events-none' : ''}`}>
              <div className="p-5 border-b border-[var(--theme-border-default)]">
                <h3 className="text-base font-semibold text-[var(--theme-text-primary)]">{t('dlp.detection_rules')}</h3>
                <p className="text-sm text-[var(--theme-text-tertiary)] mt-0.5">{t('dlp.detection_rules_desc')}</p>
              </div>
              <ul className="divide-y divide-[var(--theme-border-default)]">
                {PII_RULES.map(rule => (
                  <li key={rule.field} className="px-5 py-4 flex items-center justify-between hover:bg-[var(--theme-bg-subtle)] transition-colors">
                    <div className="flex items-center gap-3">
                      <rule.icon className="w-5 h-5" style={{ color: 'var(--theme-text-secondary)' }} />
                      <div>
                        <span className="block text-sm font-medium text-[var(--theme-text-primary)]">{rule.label}</span>
                        <span className="block text-xs text-[var(--theme-text-tertiary)]">{rule.desc}</span>
                      </div>
                    </div>
                    <label className="relative inline-flex items-center cursor-pointer">
                      <input type="checkbox" className="sr-only peer" checked={(config as any)?.[rule.field] || false} onChange={(e) => handleToggleMask(rule.field, e.target.checked)} />
                      <div className="w-10 h-[22px] bg-[var(--theme-bg-subtle)] peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-[18px] after:w-[18px] after:transition-all peer-checked:bg-green-500"></div>
                    </label>
                  </li>
                ))}
              </ul>

              {/* Custom RegEx */}
              <div className="p-5 bg-[var(--theme-bg-subtle)] border-t border-[var(--theme-border-default)]">
                <label className="block text-sm font-medium text-[var(--theme-text-secondary)] mb-2">{t('dlp.custom_regex')}</label>
                <div className="flex gap-2 mb-3">
                  <input type="text" value={customRegexInput} onChange={(e) => setCustomRegexInput(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && handleAddCustomRegex()}
                    placeholder="e.g. \b(internal_proj_\w+)\b"
                    className="flex-1 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm sm:text-sm p-2.5 focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)]" />
                  <button onClick={handleAddCustomRegex} className="px-4 py-2 bg-[var(--theme-bg-surface)] hover:bg-[var(--theme-bg-hover)] text-[var(--theme-text-secondary)] rounded-xl text-sm font-medium transition-colors border border-[var(--theme-border-default)]">{t('dlp.add')}</button>
                </div>
                <div className="space-y-2">
                  {config?.customRegex?.map((regex: string, i: number) => (
                    <div key={i} className="flex items-center justify-between bg-[var(--theme-bg-surface)] border border-[var(--theme-border-default)] px-3 py-2 rounded-lg text-sm font-mono text-[var(--theme-text-secondary)]">
                      <span className="truncate">{regex}</span>
                      <button onClick={() => handleRemoveCustomRegex(i)} className="text-[var(--theme-text-tertiary)] hover:text-red-500 ml-2 flex-shrink-0"><XMarkIcon className="w-4 h-4" /></button>
                    </div>
                  ))}
                  {!config?.customRegex?.length && <p className="text-xs text-[var(--theme-text-tertiary)]">{t('dlp.no_custom_patterns')}</p>}
                </div>
              </div>
            </div>

            {/* Admin: Publish */}
            {isAdmin && projects.length > 1 && (
              <div className="card p-5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-xl bg-purple-500/10"><SparklesIcon className="w-5 h-5 text-purple-500" /></div>
                    <div>
                      <h3 className="text-sm font-semibold text-[var(--theme-text-primary)]">{t('dlp.publish_policy')}</h3>
                      <p className="text-xs text-[var(--theme-text-tertiary)]">{t('dlp.publish_policy_desc', { count: projects.length - 1 })}</p>
                    </div>
                  </div>
                  <button onClick={handlePublishToAllProjects} disabled={saving} className="px-4 py-2 bg-purple-500 hover:bg-purple-600 text-white rounded-xl text-sm font-medium shadow-sm transition-colors disabled:opacity-50">
                    {saving ? t('dlp.publishing') : t('dlp.publish')}
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Right Column: Simulator */}
          <div className="lg:col-span-1">
            <div className="card sticky top-6 overflow-hidden">
              <div className="p-5 border-b border-[var(--theme-border-default)] bg-[var(--theme-bg-subtle)]">
                <h3 className="text-base font-semibold text-[var(--theme-text-primary)] flex items-center gap-2">
                  <BeakerIcon className="w-5 h-5 inline-block mr-1" style={{ color: 'var(--theme-text-secondary)' }} /> {t('dlp.simulator')}
                </h3>
                <p className="text-xs text-[var(--theme-text-tertiary)] mt-0.5">{t('dlp.simulator_desc')}</p>
              </div>
              <div className="p-5 space-y-4">
                <textarea value={testInput} onChange={(e) => setTestInput(e.target.value)}
                  placeholder={t('dlp.simulator_placeholder')}
                  className="w-full h-32 rounded-xl border border-[var(--theme-border-default)] bg-[var(--theme-bg-surface)] text-[var(--theme-text-primary)] shadow-sm text-sm p-3 focus:ring-[var(--theme-color-primary)] focus:border-[var(--theme-color-primary)] resize-none placeholder:text-[var(--theme-text-tertiary)]" />
                <button onClick={handleRunSandbox} disabled={testing || !testInput.trim() || !isEnabled}
                  className="w-full py-2.5 bg-[var(--theme-text-primary)] text-[var(--theme-bg-surface)] rounded-xl text-sm font-medium shadow-sm transition-all hover:opacity-90 disabled:opacity-40">
                  {testing ? t('dlp.scanning') : t('dlp.run_simulation')}
                </button>
                {testResult && (
                  <div className="pt-4 border-t border-[var(--theme-border-default)] space-y-3">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-[var(--theme-text-primary)]">{t('dlp.result_label')}</span>
                      {testResult.hasPii ? (
                        <span className="px-2.5 py-0.5 rounded-full bg-red-500/15 text-red-500 text-xs font-semibold">{t('dlp.pii_detected')}</span>
                      ) : (
                        <span className="px-2.5 py-0.5 rounded-full bg-green-500/15 text-green-500 text-xs font-semibold">{t('dlp.clean')}</span>
                      )}
                      {testResult.hasPii && testResult.blocked && (
                        <span className="px-2.5 py-0.5 rounded-full bg-red-500 text-white text-xs font-semibold">{t('dlp.blocked')}</span>
                      )}
                    </div>
                    <div className="text-sm font-mono whitespace-pre-wrap bg-[var(--theme-bg-subtle)] border border-[var(--theme-border-default)] rounded-xl p-3 text-[var(--theme-text-secondary)] min-h-[4rem] leading-relaxed">
                      {testResult.scrubbedText}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </>
      )}
    </div>
  );
}
