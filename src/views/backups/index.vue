<template>
  <div class="page">
    <PageHeader :title="$t('backups.title')" :subtitle="$t('backups.subtitle')">
      <template #actions>
        <a-button :loading="backupLoading" @click="loadBackups">{{ $t('backups.refresh') }}</a-button>
        <a-button type="primary" :loading="backupBusy" @click="createBackup">{{ $t('backups.backupNow') }}</a-button>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>{{ $t('backups.list') }}</h3>
        <span class="muted">{{ $t('backups.summary', { n: backupItems.length, size: formatBytes(backupTotalSize) }) }}</span>
      </div>
      <div class="panel-body">
        <a-alert v-if="pendingRestore" type="warning" show-icon class="restore-alert">
          <template #message>{{ $t('backups.restoreReady') }}</template>
          <template #description>
            <div class="restore-row">
              <span>{{ $t('backups.restoreReadyDesc') }}</span>
              <a-space>
                <a-button size="small" type="primary" @click="restartApp">{{ $t('backups.restartNow') }}</a-button>
                <a-button size="small" :loading="backupBusy" @click="cancelRestore">{{ $t('backups.cancelRestore') }}</a-button>
              </a-space>
            </div>
          </template>
        </a-alert>

        <div class="retention-row">
          <span class="muted">{{ $t('backups.retention') }}</span>
          <label>
            {{ $t('backups.daily') }}
            <a-input-number v-model:value="retentionForm.backupDaily" :min="1" :max="30" size="small" style="width: 70px" />
            {{ $t('backups.copies') }}
          </label>
          <label>
            {{ $t('backups.preImport') }}
            <a-input-number v-model:value="retentionForm.backupPreImport" :min="1" :max="30" size="small" style="width: 70px" />
            {{ $t('backups.copies') }}
          </label>
          <label>
            {{ $t('backups.preMigrate') }}
            <a-input-number v-model:value="retentionForm.backupPreMigrate" :min="1" :max="30" size="small" style="width: 70px" />
            {{ $t('backups.copies') }}
          </label>
          <a-button size="small" :loading="retentionSaving" @click="saveRetention">{{ $t('backups.save') }}</a-button>
        </div>

        <div class="backup-summary">
          <span class="muted">
            {{
              $t('backups.currentPolicy', {
                daily: retention.daily,
                preImport: retention['pre-import'],
                preMigrate: retention['pre-migrate'],
                preDelete: retention['pre-delete'],
                cap: formatBytes(backupCapBytes),
              })
            }}
          </span>
        </div>

        <a-spin :spinning="backupLoading">
          <div class="backup-list">
            <div v-for="item in backupItems" :key="item.name" class="backup-row">
              <a-tag :color="kindColor(item.kind)">{{ kindLabel(item.kind) }}</a-tag>
              <div class="backup-main">
                <div class="backup-name mono">{{ item.name }}</div>
                <div class="muted">{{ formatTime(item.createdAt) }} · {{ formatBytes(item.size) }}</div>
              </div>
              <a-space :size="6">
                <a-button size="small" :disabled="pendingRestore" @click="confirmRestore(item)">{{ $t('backups.restore') }}</a-button>
                <a-button size="small" danger :disabled="pendingRestore" @click="confirmDelete(item)">
                  {{ $t('backups.remove') }}
                </a-button>
              </a-space>
            </div>
            <a-empty v-if="!backupLoading && backupItems.length === 0" :description="$t('backups.empty')" :image="false" />
          </div>
        </a-spin>
        <p class="note mono">{{ backupsDir }}</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Modal, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { backupsApi, type BackupItem, type BackupOverview } from '@/api/backups'
import { settingsApi } from '@/api/settings'
import { isTauri } from '@/api/request'
import { formatDateTime } from '@/i18n'

const { t } = useI18n()

const backupOverview = ref<BackupOverview | null>(null)
const backupLoading = ref(false)
const backupBusy = ref(false)
const pendingRestore = ref(false)
const backupItems = computed(() => backupOverview.value?.items ?? [])
const backupTotalSize = computed(() => backupOverview.value?.totalSize ?? 0)
const backupCapBytes = computed(() => backupOverview.value?.capBytes ?? 0)
const retention = computed(() => backupOverview.value?.retention ?? {})
const backupsDir = computed(() => backupOverview.value?.backupsDir ?? '')

const retentionForm = reactive({ backupDaily: 7, backupPreImport: 5, backupPreMigrate: 3 })
const retentionSaving = ref(false)

const KIND_KEYS: Record<string, string> = {
  manual: 'backups.kindManual',
  daily: 'backups.kindDaily',
  'pre-import': 'backups.kindPreImport',
  'pre-migrate': 'backups.kindPreMigrate',
  'pre-restore': 'backups.kindPreRestore',
  'pre-delete': 'backups.kindPreDelete',
}

const KIND_COLORS: Record<string, string> = {
  manual: 'green',
  daily: 'blue',
  'pre-import': 'orange',
  'pre-migrate': 'purple',
  'pre-restore': 'cyan',
  'pre-delete': 'red',
}

function kindLabel(kind: string): string {
  const key = KIND_KEYS[kind]
  return key ? t(key) : kind
}

function kindColor(kind: string): string {
  return KIND_COLORS[kind] ?? 'default'
}

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatTime(value: string): string {
  return value ? formatDateTime(value) : '—'
}

async function loadBackups() {
  backupLoading.value = true
  try {
    const data = await backupsApi.list()
    backupOverview.value = data
    pendingRestore.value = data.pendingRestore
  } catch {
    backupOverview.value = null
  } finally {
    backupLoading.value = false
  }
}

// 保留策略属于全局设置：先取全量再合并提交，避免把这页不显示的练习设置覆盖掉。
async function loadRetention() {
  try {
    const data = await settingsApi.get()
    retentionForm.backupDaily = data.backupDaily
    retentionForm.backupPreImport = data.backupPreImport
    retentionForm.backupPreMigrate = data.backupPreMigrate
  } catch {
    /* 引擎未就绪时保留默认值 */
  }
}

async function saveRetention() {
  retentionSaving.value = true
  try {
    const current = await settingsApi.get()
    const saved = await settingsApi.update({ ...current, ...retentionForm })
    retentionForm.backupDaily = saved.backupDaily
    retentionForm.backupPreImport = saved.backupPreImport
    retentionForm.backupPreMigrate = saved.backupPreMigrate
    message.success(t('backups.msg.retentionSaved'))
    await loadBackups()
  } finally {
    retentionSaving.value = false
  }
}

async function createBackup() {
  backupBusy.value = true
  try {
    const info = await backupsApi.create()
    message.success(t('backups.msg.created', { name: info.name }))
    await loadBackups()
  } finally {
    backupBusy.value = false
  }
}

function confirmDelete(item: BackupItem) {
  Modal.confirm({
    title: t('backups.msg.deleteTitle'),
    content: t('backups.msg.deleteContent', { name: item.name }),
    okText: t('backups.remove'),
    okType: 'danger',
    cancelText: t('common.cancel'),
    async onOk() {
      await backupsApi.remove(item.name)
      message.success(t('backups.msg.deleted'))
      await loadBackups()
    },
  })
}

function confirmRestore(item: BackupItem) {
  Modal.confirm({
    title: t('backups.msg.restoreTitle'),
    content: t('backups.msg.restoreContent', { name: item.name }),
    okText: t('backups.msg.restoreOk'),
    cancelText: t('common.cancel'),
    async onOk() {
      backupBusy.value = true
      try {
        await backupsApi.restore(item.name)
        pendingRestore.value = true
        await loadBackups()
        Modal.confirm({
          title: t('backups.msg.restoreReadyTitle'),
          content: t('backups.msg.restoreReadyContent'),
          okText: t('backups.restartNow'),
          cancelText: t('backups.msg.restartLater'),
          onOk() {
            return restartApp()
          },
        })
      } finally {
        backupBusy.value = false
      }
    },
  })
}

async function cancelRestore() {
  backupBusy.value = true
  try {
    await backupsApi.cancelRestore()
    pendingRestore.value = false
    message.success(t('backups.msg.restoreCancelled'))
    await loadBackups()
  } finally {
    backupBusy.value = false
  }
}

async function restartApp() {
  if (!isTauri()) {
    message.info(t('backups.msg.restartManual'))
    return
  }
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('restart_app')
  } catch {
    message.error(t('backups.msg.restartFailed'))
  }
}

onMounted(() => {
  void loadBackups()
  void loadRetention()
})
</script>

<style scoped>
.restore-alert { margin-bottom: 14px; }
.restore-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.retention-row { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; font-size: 12px; margin-bottom: 10px; }
.retention-row label { display: inline-flex; align-items: center; gap: 6px; color: var(--text2); }
.backup-summary { display: grid; gap: 4px; font-size: 12px; margin-bottom: 8px; }
.backup-list { display: grid; gap: 0; }
.backup-row { display: flex; align-items: center; gap: 10px; padding: 8px 0; border-bottom: 1px solid var(--border); }
.backup-row:last-child { border-bottom: 0; }
.backup-main { flex: 1; min-width: 0; display: grid; gap: 2px; }
.backup-name { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.note { margin: 14px 0 0; font-size: 11px; color: var(--text2); }
</style>
