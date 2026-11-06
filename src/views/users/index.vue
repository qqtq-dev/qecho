<template>
  <div class="page">
    <PageHeader :title="$t('users.title')" :subtitle="$t('users.subtitle')">
      <template #actions>
        <a-button :loading="usersLoading" @click="loadUsers">{{ $t('users.refresh') }}</a-button>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>{{ $t('users.list') }}</h3>
        <span class="muted">{{ $t('users.count', { n: users.length }) }}</span>
      </div>
      <div class="panel-body">
        <a-table
          :data-source="users"
          :columns="userColumns"
          :pagination="false"
          row-key="id"
          size="small"
          :loading="usersLoading"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'username'">
              <div class="user-cell">
                <span>{{ record.username }}</span>
                <a-tag v-if="record.username === auth.username" color="green">{{ $t('users.currentAccount') }}</a-tag>
                <a-tag v-if="record.username === 'local'" color="blue">{{ $t('users.localAccount') }}</a-tag>
              </div>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-space :size="4">
                <a-tag :color="record.disabled ? 'red' : 'green'">
                  {{ record.disabled ? $t('users.disabled') : $t('users.active') }}
                </a-tag>
                <a-button
                  v-if="record.username !== 'local' && record.username !== auth.username"
                  type="link"
                  size="small"
                  :disabled="statusSaving === record.id"
                  @click="toggleStatus(record)"
                >
                  {{ record.disabled ? $t('users.enable') : $t('users.disable') }}
                </a-button>
              </a-space>
            </template>
            <template v-else-if="column.key === 'role'">
              <a-tag :color="record.role === 'admin' ? 'green' : 'default'">
                {{ record.role === 'admin' ? $t('users.roleAdmin') : $t('users.roleUser') }}
              </a-tag>
            </template>
            <template v-else-if="column.key === 'permissions'">
              <span v-if="record.role === 'admin'" class="muted">{{ $t('users.allPermissions') }}</span>
              <a-space v-else :size="14">
                <a-checkbox
                  :checked="record.canCreateBank"
                  :disabled="permSaving === record.id"
                  @change="onCreatePermChange(record, $event)"
                >
                  {{ $t('users.permCreateBank') }}
                </a-checkbox>
                <a-checkbox
                  :checked="record.canExportImport"
                  :disabled="permSaving === record.id"
                  @change="onExportPermChange(record, $event)"
                >
                  {{ $t('users.permExportImport') }}
                </a-checkbox>
              </a-space>
            </template>
            <template v-else-if="column.key === 'createdAt'">
              <span class="muted">{{ formatTime(record.createdAt) }}</span>
            </template>
            <template v-else-if="column.key === 'lastLoginAt'">
              <span class="muted">{{ record.lastLoginAt ? formatTime(record.lastLoginAt) : '—' }}</span>
            </template>
            <template v-else-if="column.key === 'actions'">
              <a-space :size="6">
                <a-button
                  size="small"
                  :disabled="record.username === 'local' || record.username === auth.username"
                  @click="toggleRole(record)"
                >
                  {{ record.role === 'admin' ? $t('users.setUser') : $t('users.setAdmin') }}
                </a-button>
                <a-button
                  size="small"
                  :disabled="record.username === 'local'"
                  @click="openReset(record)"
                >
                  {{ $t('users.resetPassword') }}
                </a-button>
                <a-popconfirm
                  :title="$t('users.removeConfirm')"
                  :ok-text="$t('users.remove')"
                  :cancel-text="$t('common.cancel')"
                  :width="320"
                  @confirm="removeUser(record)"
                >
                  <a-button
                    size="small"
                    danger
                    :disabled="
                      record.username === 'local' ||
                      record.username === auth.username ||
                      !record.disabled
                    "
                    :title="record.disabled ? '' : $t('users.removeDisabledHint')"
                  >
                    {{ $t('users.remove') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
        <p class="note">{{ $t('users.note') }}</p>
      </div>
    </section>

    <a-modal
      v-model:open="resetOpen"
      :title="$t('users.resetTitle', { name: resetUser?.username ?? '' })"
      :confirm-loading="resetSaving"
      :ok-text="$t('users.resetOk')"
      :cancel-text="$t('common.cancel')"
      :width="420"
      @ok="saveReset"
    >
      <a-form layout="vertical">
        <a-form-item :label="$t('users.newPassword')" :extra="$t('users.newPasswordExtra')">
          <a-input-password v-model:value="resetForm.password" autocomplete="new-password" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Modal, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { usersApi, type UserItem } from '@/api/users'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime } from '@/i18n'

const { t } = useI18n()
const auth = useAuthStore()

const users = ref<UserItem[]>([])
const usersLoading = ref(false)
const userColumns = computed(() => [
  { title: t('users.colUser'), key: 'username', dataIndex: 'username' },
  { title: t('users.colStatus'), key: 'status', width: 130 },
  { title: t('users.colRole'), key: 'role', width: 100 },
  { title: t('users.colPermissions'), key: 'permissions', width: 190 },
  { title: t('users.colCreated'), key: 'createdAt', width: 160 },
  { title: t('users.colLastLogin'), key: 'lastLoginAt', width: 160 },
  { title: t('users.colActions'), key: 'actions', width: 300 },
])

function formatTime(value: string): string {
  return value ? formatDateTime(value) : '—'
}

async function loadUsers() {
  usersLoading.value = true
  try {
    users.value = (await usersApi.list()).items
  } finally {
    usersLoading.value = false
  }
}

const permSaving = ref<number | null>(null)

function checkedOf(event: unknown): boolean {
  return Boolean((event as { target?: { checked?: boolean } } | undefined)?.target?.checked)
}

function onCreatePermChange(item: UserItem, event: unknown) {
  void togglePermission(item, 'canCreateBank', event)
}

function onExportPermChange(item: UserItem, event: unknown) {
  void togglePermission(item, 'canExportImport', event)
}

async function togglePermission(
  item: UserItem,
  key: 'canCreateBank' | 'canExportImport',
  event: unknown,
) {
  const next = { canCreateBank: item.canCreateBank, canExportImport: item.canExportImport }
  next[key] = checkedOf(event)
  permSaving.value = item.id
  try {
    await usersApi.updatePermissions(item.id, next)
    item.canCreateBank = next.canCreateBank
    item.canExportImport = next.canExportImport
    message.success(t('users.msg.permissionsSaved'))
  } catch {
    await loadUsers()
  } finally {
    permSaving.value = null
  }
}

const statusSaving = ref<number | null>(null)

async function toggleStatus(item: UserItem) {
  const disable = !item.disabled
  statusSaving.value = item.id
  try {
    await usersApi.setStatus(item.id, disable)
    item.disabled = disable
    message.success(
      disable ? t('users.msg.disabled', { name: item.username }) : t('users.msg.enabled', { name: item.username }),
    )
  } catch {
    await loadUsers()
  } finally {
    statusSaving.value = null
  }
}

function removeUser(item: UserItem) {
  Modal.confirm({
    title: t('users.msg.deleteTitle'),
    content: t('users.msg.deleteContent', { name: item.username }),
    okText: t('users.remove'),
    okType: 'danger',
    cancelText: t('common.cancel'),
    async onOk() {
      await usersApi.remove(item.id)
      message.success(t('users.msg.deleted', { name: item.username }))
      await loadUsers()
    },
  })
}

function toggleRole(item: UserItem) {
  const target: 'admin' | 'user' = item.role === 'admin' ? 'user' : 'admin'
  Modal.confirm({
    title: target === 'admin' ? t('users.msg.roleTitleAdmin') : t('users.msg.roleTitleUser'),
    content:
      target === 'admin'
        ? t('users.msg.roleContentAdmin', { name: item.username })
        : t('users.msg.roleContentUser', { name: item.username }),
    okText: t('users.msg.confirm'),
    cancelText: t('common.cancel'),
    async onOk() {
      await usersApi.updateRole(item.id, target)
      message.success(t('users.msg.roleUpdated'))
      await loadUsers()
    },
  })
}

const resetOpen = ref(false)
const resetSaving = ref(false)
const resetUser = ref<UserItem | null>(null)
const resetForm = reactive({ password: '' })

function openReset(item: UserItem) {
  resetUser.value = item
  resetForm.password = ''
  resetOpen.value = true
}

async function saveReset() {
  if (!resetUser.value) return
  if (resetForm.password.length < 8 || resetForm.password.length > 72) {
    message.warning(t('users.msg.pwdLength'))
    return
  }
  resetSaving.value = true
  try {
    await usersApi.resetPassword(resetUser.value.id, resetForm.password)
    message.success(t('users.msg.pwdReset', { name: resetUser.value.username }))
    resetOpen.value = false
  } finally {
    resetSaving.value = false
  }
}

onMounted(() => {
  void loadUsers()
})
</script>

<style scoped>
.user-cell { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.note { margin: 14px 0 0; font-size: 11px; color: var(--text2); line-height: 1.7; }
</style>
