<template>
  <div class="page">
    <PageHeader title="用户管理" subtitle="账号状态、角色，以及建题库与导出导入权限">
      <template #actions>
        <a-button :loading="usersLoading" @click="loadUsers">刷新</a-button>
      </template>
    </PageHeader>

    <section class="panel">
      <div class="panel-head">
        <h3>账号列表</h3>
        <span class="muted">共 {{ users.length }} 个</span>
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
                <a-tag v-if="record.username === auth.username" color="green">当前账号</a-tag>
                <a-tag v-if="record.username === 'local'" color="blue">本机</a-tag>
              </div>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-space :size="4">
                <a-tag :color="record.disabled ? 'red' : 'green'">
                  {{ record.disabled ? '已停用' : '正常' }}
                </a-tag>
                <a-button
                  v-if="record.username !== 'local' && record.username !== auth.username"
                  type="link"
                  size="small"
                  :disabled="statusSaving === record.id"
                  @click="toggleStatus(record)"
                >
                  {{ record.disabled ? '启用' : '停用' }}
                </a-button>
              </a-space>
            </template>
            <template v-else-if="column.key === 'role'">
              <a-tag :color="record.role === 'admin' ? 'green' : 'default'">
                {{ record.role === 'admin' ? '管理员' : '普通用户' }}
              </a-tag>
            </template>
            <template v-else-if="column.key === 'permissions'">
              <span v-if="record.role === 'admin'" class="muted">全部权限</span>
              <a-space v-else :size="14">
                <a-checkbox
                  :checked="record.canCreateBank"
                  :disabled="permSaving === record.id"
                  @change="onCreatePermChange(record, $event)"
                >
                  建题库
                </a-checkbox>
                <a-checkbox
                  :checked="record.canExportImport"
                  :disabled="permSaving === record.id"
                  @change="onExportPermChange(record, $event)"
                >
                  导出导入
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
                  {{ record.role === 'admin' ? '设为普通用户' : '设为管理员' }}
                </a-button>
                <a-button
                  size="small"
                  :disabled="record.username === 'local'"
                  @click="openReset(record)"
                >
                  重置密码
                </a-button>
                <a-popconfirm
                  title="删除账号后，其个人题库会一并删除，创建的外联考试会被关闭（删除前自动备份）。确定吗？"
                  ok-text="删除"
                  cancel-text="取消"
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
                    :title="record.disabled ? '' : '请先停用账号，确认无需恢复后再删除'"
                  >
                    删除
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
        <p class="note">
          重置密码会让该用户已登录的会话失效；本机账号请到「设置 → 账号安全」修改自己的密码。
          「建题库」「导出导入」为普通用户的授权位，管理员始终具备；删除账号前会自动备份数据库。
          删除不可撤回，想暂时禁止登录请用「停用」。
        </p>
      </div>
    </section>

    <a-modal
      v-model:open="resetOpen"
      :title="`重置密码：${resetUser?.username ?? ''}`"
      :confirm-loading="resetSaving"
      ok-text="重置"
      cancel-text="取消"
      :width="420"
      @ok="saveReset"
    >
      <a-form layout="vertical">
        <a-form-item label="新密码" extra="8–72 位；重置后该用户需用新密码重新登录">
          <a-input-password v-model:value="resetForm.password" autocomplete="new-password" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Modal, message } from 'ant-design-vue'
import PageHeader from '@/components/PageHeader.vue'
import { usersApi, type UserItem } from '@/api/users'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()

const users = ref<UserItem[]>([])
const usersLoading = ref(false)
const userColumns = [
  { title: '用户', key: 'username', dataIndex: 'username' },
  { title: '状态', key: 'status', width: 130 },
  { title: '角色', key: 'role', width: 100 },
  { title: '权限', key: 'permissions', width: 190 },
  { title: '注册时间', key: 'createdAt', width: 160 },
  { title: '最近登录', key: 'lastLoginAt', width: 160 },
  { title: '操作', key: 'actions', width: 300 },
]

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
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
    message.success('权限已更新')
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
    message.success(disable ? `已停用 ${item.username}，其登录会话已失效` : `已启用 ${item.username}`)
  } catch {
    await loadUsers()
  } finally {
    statusSaving.value = null
  }
}

function removeUser(item: UserItem) {
  Modal.confirm({
    title: '删除账号',
    content: `删除 ${item.username} 后，其个人题库会一并删除，创建的外联考试会被关闭。删除前会自动备份，确定继续？`,
    okText: '删除',
    okType: 'danger',
    cancelText: '取消',
    async onOk() {
      await usersApi.remove(item.id)
      message.success(`已删除 ${item.username}`)
      await loadUsers()
    },
  })
}

function toggleRole(item: UserItem) {
  const target: 'admin' | 'user' = item.role === 'admin' ? 'user' : 'admin'
  Modal.confirm({
    title: target === 'admin' ? '设为管理员' : '取消管理员',
    content:
      target === 'admin'
        ? `将 ${item.username} 设为管理员，可管理用户、全部题库与全局设置。`
        : `将 ${item.username} 设为普通用户，之后只能访问自己的题库与被共享的题库；建题库与导出导入权限需在列表中单独授予。`,
    okText: '确定',
    cancelText: '取消',
    async onOk() {
      await usersApi.updateRole(item.id, target)
      message.success('角色已更新')
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
    message.warning('密码长度需为 8–72 位')
    return
  }
  resetSaving.value = true
  try {
    await usersApi.resetPassword(resetUser.value.id, resetForm.password)
    message.success(`已重置 ${resetUser.value.username} 的密码`)
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
