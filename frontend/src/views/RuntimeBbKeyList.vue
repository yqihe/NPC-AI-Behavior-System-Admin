<template>
  <div class="list-root">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">运行时 BB Key 管理</h2>
        <span class="page-subtitle">共 {{ total }} 条，对齐游戏服务端 blackboard/keys.go 31 条内置 Key</span>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/runtime-bb-keys/create')">
          <el-icon><Plus /></el-icon>
          新建运行时 Key
        </el-button>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-input
        v-model="query.name"
        placeholder="搜索英文标识"
        clearable
        class="filter-item"
        @keyup.enter="handleSearch"
      />
      <el-input
        v-model="query.label"
        placeholder="搜索中文标签"
        clearable
        class="filter-item"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.type"
        placeholder="类型"
        clearable
        class="filter-item"
      >
        <el-option
          v-for="item in RUNTIME_BB_KEY_TYPES"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </el-select>
      <el-select
        v-model="query.group_name"
        placeholder="分组"
        clearable
        class="filter-item"
      >
        <el-option
          v-for="item in RUNTIME_BB_KEY_GROUPS"
          :key="item.value"
          :label="`${item.value} — ${item.label}`"
          :value="item.value"
        />
      </el-select>
      <el-select
        v-model="query.enabled"
        placeholder="启用状态"
        clearable
        class="filter-item"
      >
        <el-option label="已启用" :value="true" />
        <el-option label="已禁用" :value="false" />
      </el-select>
      <el-button type="primary" @click="handleSearch">
        <el-icon><Search /></el-icon>
        搜索
      </el-button>
      <el-button @click="handleReset">重置</el-button>
    </div>

    <!-- 数据表格 -->
    <div class="table-wrap">
      <el-table
        v-loading="loading"
        :data="tableData"
        :row-class-name="rowClassName"
        style="width: 100%"
      >
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="英文标识" min-width="160" />
        <el-table-column prop="label" label="中文标签" min-width="140" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="typeBadgeType(row.type)">
              {{ row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="分组" width="120">
          <template #default="{ row }">
            <el-tag size="small" type="info">
              {{ groupLabel(row.group_name) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enabled"
              @change="(val: string | number | boolean) => handleToggle(row, Boolean(val))"
            />
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click="router.push(`/runtime-bb-keys/${row.id}/view`)">查看</el-link>
            <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
            <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无运行时 Key 数据">
            <el-button type="primary" @click="$router.push('/runtime-bb-keys/create')">
              新建运行时 Key
            </el-button>
          </el-empty>
        </template>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrap">
        <span class="total-text">共 {{ total }} 条</span>
        <el-pagination
          v-model:current-page="query.page"
          :page-size="query.page_size"
          :total="total"
          layout="prev, pager, next"
          @current-change="fetchList"
        />
      </div>
    </div>

    <!-- 启用守卫弹窗（编辑/删除启用中 key 的拦截） -->
    <EnabledGuardDialog ref="guardRef" @refresh="fetchList" />

    <!-- 引用详情弹窗 -->
    <el-dialog
      v-model="refDialog.visible"
      :title="`引用详情 — ${refDialog.label} (${refDialog.name})`"
      width="500px"
      @close="resetRefDialog"
    >
      <div v-loading="refDialog.loading">
        <!-- FSM 引用 -->
        <div class="ref-section">
          <p class="ref-subtitle">
            FSM 引用（{{ refDialog.fsms.length }} 个状态机引用了该 Key）：
          </p>
          <el-table
            v-if="refDialog.fsms.length > 0"
            :data="refDialog.fsms"
            size="small"
          >
            <el-table-column prop="label" label="状态机名称" />
            <el-table-column prop="ref_type" label="类型" width="100" />
          </el-table>
          <p v-else class="ref-empty">暂无 FSM 引用</p>
        </div>

        <!-- BT 引用 -->
        <div class="ref-section" style="margin-top: 16px">
          <p class="ref-subtitle">
            行为树引用（{{ refDialog.bts.length }} 个行为树引用了该 Key）：
          </p>
          <el-table
            v-if="refDialog.bts.length > 0"
            :data="refDialog.bts"
            size="small"
          >
            <el-table-column prop="label" label="行为树名称" />
            <el-table-column prop="ref_type" label="类型" width="100" />
          </el-table>
          <p v-else class="ref-empty">暂无行为树引用</p>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import {
  runtimeBbKeyApi,
  RUNTIME_BB_KEY_ERR,
  RUNTIME_BB_KEY_TYPES,
  RUNTIME_BB_KEY_GROUPS,
} from '@/api/runtimeBbKeys'
import type { RuntimeBbKeyListItem, RuntimeBbKeyListQuery } from '@/api/runtimeBbKeys'
import type { ReferenceItem } from '@/api/fields'
import type { BizError } from '@/api/request'
import { formatTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const tableData = ref<RuntimeBbKeyListItem[]>([])
const total = ref(0)
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const query = reactive<RuntimeBbKeyListQuery>({
  name: '',
  label: '',
  type: '',
  group_name: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

const refDialog = reactive({
  visible: false,
  loading: false,
  name: '',
  label: '',
  fsms: [] as ReferenceItem[],
  bts: [] as ReferenceItem[],
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: RuntimeBbKeyListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.name) params.name = query.name
    if (query.label) params.label = query.label
    if (query.type) params.type = query.type
    if (query.group_name) params.group_name = query.group_name
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await runtimeBbKeyApi.list(params)
    tableData.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchList()
})

// ---------- 筛选 ----------

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.name = ''
  query.label = ''
  query.type = ''
  query.group_name = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: RuntimeBbKeyListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用运行时 Key「${row.label}」？启用后可被 FSM / 行为树引用。`
    : `确认禁用运行时 Key「${row.label}」？禁用后新 FSM / 行为树无法引用该 Key，已有引用不受影响。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    // 列表接口不返回 version，先获取详情拿到最新 version
    const detail = await runtimeBbKeyApi.detail(row.id)
    await runtimeBbKeyApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === RUNTIME_BB_KEY_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: RuntimeBbKeyListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'edit', entityType: 'runtime-bb-key', entity: row })
    return
  }
  router.push(`/runtime-bb-keys/${row.id}/edit`)
}

async function handleDelete(row: RuntimeBbKeyListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'delete', entityType: 'runtime-bb-key', entity: row })
    return
  }
  // 已禁用：先查引用，有引用弹详情阻止，无引用确认删除
  try {
    const res = await runtimeBbKeyApi.references(row.id)
    const fsms = res.data?.fsms || []
    const bts = res.data?.bts || []
    const refTotal = fsms.length + bts.length
    if (refTotal > 0) {
      showRefDialog(row, fsms, bts)
      ElMessage.warning(`该 Key 被 ${refTotal} 处引用，无法删除。请先移除引用关系。`)
      return
    }
  } catch {
    // references API 失败拦截器已 toast；为安全起见不继续删除
    return
  }
  // 无引用：确认删除
  try {
    await ElMessageBox.confirm(
      `确认删除运行时 Key「${row.label}」（${row.name}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await runtimeBbKeyApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    // 后端兜底：HAS_REFS 时重新拉引用详情展示
    if ((err as BizError).code === RUNTIME_BB_KEY_ERR.HAS_REFS) {
      await loadAndShowRefs(row)
    }
    // 其他错误拦截器已 toast
  }
}

function showRefDialog(
  row: RuntimeBbKeyListItem,
  fsms: ReferenceItem[],
  bts: ReferenceItem[],
) {
  refDialog.visible = true
  refDialog.loading = false
  refDialog.name = row.name
  refDialog.label = row.label
  refDialog.fsms = fsms
  refDialog.bts = bts
}

async function loadAndShowRefs(row: RuntimeBbKeyListItem) {
  refDialog.visible = true
  refDialog.loading = true
  refDialog.name = row.name
  refDialog.label = row.label
  refDialog.fsms = []
  refDialog.bts = []
  try {
    const res = await runtimeBbKeyApi.references(row.id)
    refDialog.fsms = res.data?.fsms || []
    refDialog.bts = res.data?.bts || []
  } catch {
    // 拦截器已 toast
  } finally {
    refDialog.loading = false
  }
}

function resetRefDialog() {
  refDialog.loading = false
  refDialog.name = ''
  refDialog.label = ''
  refDialog.fsms = []
  refDialog.bts = []
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: RuntimeBbKeyListItem }) {
  return row.enabled ? '' : 'row-disabled'
}

function typeBadgeType(type: string) {
  const map: Record<string, string> = {
    integer: '',
    float: '',
    string: 'success',
    bool: 'warning',
  }
  return map[type] || 'info'
}

function groupLabel(group: string) {
  const g = RUNTIME_BB_KEY_GROUPS.find((x) => x.value === group)
  return g ? g.label : group
}
</script>

<style scoped>
.page-subtitle {
  margin-left: 12px;
  font-size: 12px;
  color: #909399;
}

.ref-empty {
  color: #C0C4CC;
  font-size: 13px;
  margin: 4px 0 0 0;
}
</style>
