<template>
  <div class="page">
    <PageHeader title="数据备份" subtitle="自动备份保留策略、手动备份与恢复">
      <template #actions>
        <a-button :loading="backupLoading" @click="loadBackups">刷新</a-button>
        <a-button type="primary" :loading="backupBusy" @click="createBackup">立即备份</a-button>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>备份列表</h3>
        <span class="muted">共 {{ backupItems.length }} 份 · 占用 {{ formatBytes(backupTotalSize) }}</span>
      </div>
      <div class="panel-body">
        <a-alert v-if="pendingRestore" type="warning" show-icon class="restore-alert">
          <template #message>数据恢复已就绪</template>
          <template #description>
            <div class="restore-row">
              <span>重启应用后将用所选备份替换当前数据库，重启前无法写入新数据。</span>
              <a-space>
                <a-button size="small" type="primary" @click="restartApp">立即重启</a-button>
                <a-button size="small" :loading="backupBusy" @click="cancelRestore">取消恢复</a-button>
              </a-space>
            </div>
          </template>
        </a-alert>

        <div class="retention-row">
          <span class="muted">自动备份保留</span>
          <label>
            每日
            <a-input-number v-model:value="retentionForm.backupDaily" :min="1" :max="30" size="small" style="width: 70px" />
            份
          </label>
          <label>
            导入前
            <a-input-number v-model:value="retentionForm.backupPreImport" :min="1" :max="30" size="small" style="width: 70px" />
            份
          </label>
          <label>
            升级前
            <a-input-number v-model:value="retentionForm.backupPreMigrate" :min="1" :max="30" size="small" style="width: 70px" />
            份
          </label>
          <a-button size="small" :loading="retentionSaving" @click="saveRetention">保存</a-button>
        </div>

        <div class="backup-summary">
          <span class="muted">
            当前生效：每日 {{ retention.daily }} 份 · 导入前 {{ retention['pre-import'] }} 份 · 升级前
            {{ retention['pre-migrate'] }} 份 · 删除账号前 {{ retention['pre-delete'] }} 份；自动备份合计超过
            {{ formatBytes(backupCapBytes) }} 时清理最旧。
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
                <a-button size="small" :disabled="pendingRestore" @click="confirmRestore(item)">恢复</a-button>
                <a-button size="small" danger :disabled="pendingRestore" @click="confirmDelete(item)">
                  删除
                </a-button>
              </a-space>
            </div>
            <a-empty v-if="!backupLoading && backupItems.length === 0" description="还没有备份" :image="false" />
          </div>
        </a-spin>
        <p class="note mono">{{ backupsDir }}</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Modal, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { backupsApi, type BackupItem, type BackupOverview } from '@/api/backups'
import { settingsApi } from '@/api/settings'
import { isTauri } from '@/api/request'

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

const KIND_LABELS: Record<string, string> = {
  manual: '手动',
  daily: '每日自动',
  'pre-import': '导入前自动',
  'pre-migrate': '升级前自动',
  'pre-restore': '恢复前自动',
  'pre-delete': '删除账号前自动',
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
  return KIND_LABELS[kind] ?? kind
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
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
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
    message.success('保留策略已保存')
    await loadBackups()
  } finally {
    retentionSaving.value = false
  }
}

async function createBackup() {
  backupBusy.value = true
  try {
    const info = await backupsApi.create()
    message.success(`已创建备份 ${info.name}`)
    await loadBackups()
  } finally {
    backupBusy.value = false
  }
}

function confirmDelete(item: BackupItem) {
  Modal.confirm({
    title: '删除备份',
    content: `确定删除 ${item.name}？删除后无法恢复。`,
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      await backupsApi.remove(item.name)
      message.success('备份已删除')
      await loadBackups()
    },
  })
}

function confirmRestore(item: BackupItem) {
  Modal.confirm({
    title: '恢复数据',
    content: `将用 ${item.name} 替换当前数据库，恢复前会自动备份当前数据。恢复需要重启应用后生效。`,
    okText: '准备恢复',
    cancelText: '取消',
    async onOk() {
      backupBusy.value = true
      try {
        await backupsApi.restore(item.name)
        pendingRestore.value = true
        await loadBackups()
        Modal.confirm({
          title: '恢复已就绪',
          content: '重启应用后完成数据恢复；重启前应用不再接受新数据。',
          okText: '立即重启',
          cancelText: '稍后重启',
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
    message.success('已取消恢复')
    await loadBackups()
  } finally {
    backupBusy.value = false
  }
}

async function restartApp() {
  if (!isTauri()) {
    message.info('请关闭并重新启动应用或引擎以完成恢复')
    return
  }
  try {
    const { invoke } = await import('@tauri-apps/api/core')
    await invoke('restart_app')
  } catch {
    message.error('重启失败，请手动关闭并重新打开应用')
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
