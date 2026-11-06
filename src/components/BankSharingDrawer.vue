<template>
  <a-drawer
    :open="open"
    :width="540"
    :title="$t('sharing.title')"
    placement="right"
    destroy-on-close
    @close="close"
  >
    <a-spin :spinning="loading">
      <template v-if="state">
        <div class="head">
          <div class="bank-name">{{ bankName || $t('sharing.bankFallback') }}</div>
          <div class="muted">{{ $t('sharing.owner', { name: state.ownerName || '—' }) }}</div>
        </div>

        <a-divider class="divider" />

        <div class="block">
          <div class="block-title">{{ $t('sharing.visibility') }}</div>
          <a-radio-group
            :value="state.visibility"
            :disabled="busy"
            button-style="solid"
            @change="onVisibilityChange"
          >
            <a-radio-button value="private">{{ $t('sharing.privateOnly') }}</a-radio-button>
            <a-radio-button value="public">{{ $t('sharing.publicRead') }}</a-radio-button>
          </a-radio-group>
          <p class="hint">
            <template v-if="state.visibility === 'public'">{{ $t('sharing.publicHint') }}</template>
            <template v-else>{{ $t('sharing.privateHint') }}</template>
          </p>
        </div>

        <a-divider class="divider" />

        <div class="block">
          <div class="block-title">
            {{ $t('sharing.members') }}
            <span class="muted">{{ $t('sharing.memberCount', { n: state.members.length }) }}</span>
          </div>
          <a-empty
            v-if="!state.members.length"
            :description="$t('sharing.noMembers')"
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
                  <a-select-option value="viewer">{{ $t('sharing.viewer') }}</a-select-option>
                  <a-select-option value="editor">{{ $t('sharing.editor') }}</a-select-option>
                </a-select>
              </template>
              <template v-else-if="column.key === 'actions'">
                <a-popconfirm
                  :title="$t('sharing.removeConfirm', { name: record.username })"
                  :ok-text="$t('sharing.removeOk')"
                  :cancel-text="$t('sharing.keep')"
                  @confirm="removeMember(record)"
                >
                  <a-button type="link" size="small" danger>{{ $t('sharing.remove') }}</a-button>
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
              :not-found-content="searchHint || undefined"
              :placeholder="$t('sharing.searchPlaceholder')"
              style="flex: 1; min-width: 0"
              @search="onSearch"
            />
            <a-select v-model:value="pickLevel" style="width: 104px">
              <a-select-option value="viewer">{{ $t('sharing.viewer') }}</a-select-option>
              <a-select-option value="editor">{{ $t('sharing.editor') }}</a-select-option>
            </a-select>
            <a-button type="primary" :loading="busy" :disabled="!pickUserId" @click="addMember">
              {{ $t('sharing.add') }}
            </a-button>
          </div>
          <p class="hint">{{ $t('sharing.editorHint') }}</p>
        </div>
      </template>
    </a-spin>
  </a-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()

const state = ref<BankSharing | null>(null)
const loading = ref(false)
const busy = ref(false)
const searching = ref(false)
const searchHint = ref('')
const pickUserId = ref<number | undefined>(undefined)
const pickLevel = ref<'viewer' | 'editor'>('viewer')
const options = ref<{ value: number; label: string; disabled?: boolean }[]>([])

const columns = computed(() => [
  { title: t('sharing.colMember'), key: 'username', dataIndex: 'username' },
  { title: t('sharing.colLevel'), key: 'level', width: 130 },
  { title: '', key: 'actions', width: 80 },
])

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
    message.error((err as Error)?.message || t('sharing.msg.loadFailed'))
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
    message.success(
      value === 'public' ? t('sharing.msg.openedPublic') : t('sharing.msg.changedPrivate'),
    )
  } catch (err) {
    message.error((err as Error)?.message || t('sharing.msg.saveFailed'))
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
    message.success(t('sharing.msg.permissionsSaved'))
  } catch (err) {
    message.error((err as Error)?.message || t('sharing.msg.saveFailed'))
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
    message.success(t('sharing.msg.memberAdded'))
    pickUserId.value = undefined
    options.value = []
  } catch (err) {
    message.error((err as Error)?.message || t('sharing.msg.addFailed'))
  } finally {
    busy.value = false
  }
}

async function removeMember(record: { userId: number; username: string }) {
  if (!props.bankId) return
  busy.value = true
  try {
    apply(await banksApi.removeMember(props.bankId, record.userId))
    message.success(t('sharing.msg.memberRemoved', { name: record.username }))
  } catch (err) {
    message.error((err as Error)?.message || t('sharing.msg.actionFailed'))
  } finally {
    busy.value = false
  }
}

function onSearch(keyword: string) {
  if (searchTimer) clearTimeout(searchTimer)
  const key = keyword.trim()
  // 服务端要求至少两个字；这里提前拦一下，省一次往返也避免用户以为搜不到人。
  if (key.length < 2) {
    options.value = []
    searching.value = false
    searchHint.value = key ? t('sharing.searchHint') : ''
    return
  }
  searchHint.value = ''
  searching.value = true
  searchTimer = setTimeout(async () => {
    try {
      const res = await usersApi.search(key)
      const taken = new Set((state.value?.members ?? []).map((m) => m.userId))
      options.value = res.items
        .filter((u) => u.role !== 'admin')
        .map((u) => ({
          value: u.id,
          label: taken.has(u.id) ? t('sharing.memberShared', { name: u.username }) : u.username,
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
