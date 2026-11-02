<template>
  <a-drawer
    :open="open"
    :width="540"
    title="共享题库"
    placement="right"
    destroy-on-close
    @close="close"
  >
    <a-spin :spinning="loading">
      <template v-if="state">
        <div class="head">
          <div class="bank-name">{{ bankName || '题库' }}</div>
          <div class="muted">所有者：{{ state.ownerName || '—' }}</div>
        </div>

        <a-divider class="divider" />

        <div class="block">
          <div class="block-title">可见范围</div>
          <a-radio-group
            :value="state.visibility"
            :disabled="busy"
            button-style="solid"
            @change="onVisibilityChange"
          >
            <a-radio-button value="private">仅共享成员</a-radio-button>
            <a-radio-button value="public">所有登录用户（只读）</a-radio-button>
          </a-radio-group>
          <p class="hint">
            <template v-if="state.visibility === 'public'">
              全部登录用户都能看到并练习这个题库，但只有你与共享成员可以修改内容。
            </template>
            <template v-else>
              只有你与下方共享成员能看到这个题库。
            </template>
          </p>
        </div>

        <a-divider class="divider" />

        <div class="block">
          <div class="block-title">
            共享成员
            <span class="muted">共 {{ state.members.length }} 人</span>
          </div>
          <a-empty
            v-if="!state.members.length"
            description="还没有共享给任何人"
            :image="false"
          />
          <a-table
            v-else
            :data-source="state.members"
            :columns="columns"
            :pagination="false"
            row-key="userId"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'username'">
                <span>{{ record.username }}</span>
              </template>
              <template v-else-if="column.key === 'level'">
                <a-select
                  :value="record.level"
                  size="small"
                  style="width: 110px"
                  :disabled="busy"
                  @change="(value: string) => onLevelChange(record, value)"
                >
                  <a-select-option value="viewer">只读</a-select-option>
                  <a-select-option value="editor">可编辑</a-select-option>
                </a-select>
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-popconfirm
                  :title="`取消与 ${record.username} 的共享？`"
                  ok-text="取消共享"
                  cancel-text="保留"
                  @confirm="removeMember(record)"
                >
                  <a-button type="link" size="small" danger>移除</a-button>
                </a-popconfirm>
              </template>
            </template>
          </a-table>

          <div class="add-row">
            <a-select
              v-model:value="pickUserId"
              show-search
              :filter-option="false"
              :options="options"
              :loading="searching"
              placeholder="输入用户名搜索"
              style="flex: 1; min-width: 0"
              @search="onSearch"
            />
            <a-select v-model:value="pickLevel" style="width: 104px">
              <a-select-option value="viewer">只读</a-select-option>
              <a-select-option value="editor">可编辑</a-select-option>
            </a-select>
            <a-button type="primary" :loading="busy" :disabled="!pickUserId" @click="addMember">
              添加
            </a-button>
          </div>
          <p class="hint">可编辑的成员能增删改题目与导入题目，但不能改题库信息、删除题库或管理标签。</p>
        </div>
      </template>
    </a-spin>
  </a-drawer>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { banksApi, type BankSharing } from '@/api/banks'
import { usersApi } from '@/api/users'

const props = defineProps<{
  open: boolean
  bankId: number | null
  bankName?: string
}>()

const emit = defineEmits<{
  (e: 'updated'): void
  (e: 'update:open', value: boolean): void
}>()

const state = ref<BankSharing | null>(null)
const loading = ref(false)
const busy = ref(false)
const searching = ref(false)
const pickUserId = ref<number | undefined>(undefined)
const pickLevel = ref<'viewer' | 'editor'>('viewer')
const options = ref<{ value: number; label: string; disabled?: boolean }[]>([])

const columns = [
  { title: '成员', key: 'username', dataIndex: 'username' },
  { title: '权限', key: 'level', width: 130 },
  { title: '', key: 'actions', width: 80 },
]

let searchTimer: ReturnType<typeof setTimeout> | undefined

function apply(next: BankSharing) {
  state.value = next
  emit('updated')
}

async function load() {
  if (!props.bankId) return
  loading.value = true
  try {
    state.value = await banksApi.sharing(props.bankId)
  } catch (err) {
    message.error((err as Error)?.message || '读取共享设置失败')
  } finally {
    loading.value = false
  }
}

async function onVisibilityChange(event: unknown) {
  const value = (event as { target: { value: string } }).target.value
  if (!props.bankId || value === state.value?.visibility) return
  busy.value = true
  try {
    apply(await banksApi.updateSharing(props.bankId, value as 'private' | 'public'))
    message.success(value === 'public' ? '已对所有登录用户开放（只读）' : '已改为仅共享成员可见')
  } catch (err) {
    message.error((err as Error)?.message || '保存失败')
    await load()
  } finally {
    busy.value = false
  }
}

async function onLevelChange(record: { userId: number }, level: string) {
  if (!props.bankId) return
  busy.value = true
  try {
    apply(await banksApi.saveMember(props.bankId, record.userId, level as 'viewer' | 'editor'))
    message.success('权限已更新')
  } catch (err) {
    message.error((err as Error)?.message || '保存失败')
    await load()
  } finally {
    busy.value = false
  }
}

async function addMember() {
  if (!props.bankId || !pickUserId.value) return
  busy.value = true
  try {
    apply(await banksApi.saveMember(props.bankId, pickUserId.value, pickLevel.value))
    message.success('已添加共享成员')
    pickUserId.value = undefined
    options.value = []
  } catch (err) {
    message.error((err as Error)?.message || '添加失败')
  } finally {
    busy.value = false
  }
}

async function removeMember(record: { userId: number; username: string }) {
  if (!props.bankId) return
  busy.value = true
  try {
    apply(await banksApi.removeMember(props.bankId, record.userId))
    message.success(`已取消与 ${record.username} 的共享`)
  } catch (err) {
    message.error((err as Error)?.message || '操作失败')
  } finally {
    busy.value = false
  }
}

function onSearch(keyword: string) {
  if (searchTimer) clearTimeout(searchTimer)
  const key = keyword.trim()
  if (!key) {
    options.value = []
    return
  }
  searching.value = true
  searchTimer = setTimeout(async () => {
    try {
      const res = await usersApi.search(key)
      const taken = new Set((state.value?.members ?? []).map((m) => m.userId))
      options.value = res.items
        .filter((u) => u.role !== 'admin')
        .map((u) => ({
          value: u.id,
          label: taken.has(u.id) ? `${u.username}（已共享）` : u.username,
          disabled: taken.has(u.id),
        }))
    } catch {
      options.value = []
    } finally {
      searching.value = false
    }
  }, 250)
}

function close() {
  pickUserId.value = undefined
  options.value = []
  emit('update:open', false)
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      state.value = null
      pickUserId.value = undefined
      options.value = []
      void load()
    }
  },
)

defineExpose({ close })
</script>

<style scoped>
.head { display: flex; flex-direction: column; gap: 2px; }
.bank-name { font-size: 16px; font-weight: 600; }
.divider { margin: 16px 0; }
.block { display: flex; flex-direction: column; gap: 10px; }
.block-title { display: flex; align-items: baseline; gap: 8px; font-weight: 600; }
.hint { margin: 0; font-size: 12px; color: var(--muted-text, #8a8a8a); line-height: 1.6; }
.add-row { display: flex; gap: 8px; align-items: center; margin-top: 10px; }
</style>
