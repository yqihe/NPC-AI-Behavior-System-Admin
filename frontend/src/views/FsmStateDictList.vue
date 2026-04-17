<template>
  <div class="list-root">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">状态字典管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/fsm-state-dicts/create')">
          <el-icon><Plus /></el-icon>
          新建状态
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
        v-model="query.display_name"
        placeholder="搜索中文标签"
        clearable
        class="filter-item"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.category"
        placeholder="状态分类"
        clearable
        class="filter-item"
      >
        <el-option
          v-for="item in categoryOptions"
          :key="item.name"
          :label="item.label"
          :value="item.name"
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
        <el-table-column prop="name" label="状态标识" min-width="160" />
        <el-table-column prop="display_name" label="中文标签" min-width="140" />
        <el-table-column label="分类" width="120">
          <template #default="{ row }">
            <el-tag size="small" type="info">
              {{ row.category_label || row.category }}
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
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click="router.push(`/fsm-state-dicts/${row.id}/view`)">查看</el-link>
            <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
            <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无状态字典数据">
            <el-button type="primary" @click="$router.push('/fsm-state-dicts/create')">
              新建状态
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

    <!-- 启用守卫弹窗 -->
    <EnabledGuardDialog ref="guardRef" @refresh="fetchList" />

    <!-- 引用详情弹窗 -->
    <el-dialog
      v-model="refDialog.visible"
      :title="`引用详情 — ${refDialog.label} (${refDialog.name})`"
      width="540px"
      @close="resetRefDialog"
    >
      <div v-loading="refDialog.loading">
        <div class="ref-section">
          <p class="ref-subtitle">
            FSM 引用（{{ refDialog.fsmConfigs.length }} 个状态机使用了该状态）：
          </p>
          <el-table
            v-if="refDialog.fsmConfigs.length > 0"
            :data="refDialog.fsmConfigs"
            size="small"
          >
            <el-table-column prop="name" label="状态机标识" min-width="140" />
            <el-table-column prop="display_name" label="中文标签" min-width="140" />
            <el-table-column label="启用" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                  {{ row.enabled ? '启用' : '停用' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
          <p v-else class="ref-empty">暂无 FSM 引用</p>
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
import { fsmStateDictApi, FSM_STATE_DICT_ERR } from '@/api/fsmStateDicts'
import type {
  FsmStateDictListItem,
  FsmStateDictListQuery,
  FsmStateDictRefConfigItem,
} from '@/api/fsmStateDicts'
import type { BizError } from '@/api/request'
import { dictApi } from '@/api/dictionaries'
import type { DictionaryItem } from '@/api/dictionaries'
import { formatTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const tableData = ref<FsmStateDictListItem[]>([])
const total = ref(0)
const categoryOptions = ref<DictionaryItem[]>([])
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const refDialog = reactive({
  visible: false,
  loading: false,
  name: '',
  label: '',
  fsmConfigs: [] as FsmStateDictRefConfigItem[],
})

const query = reactive<FsmStateDictListQuery>({
  name: '',
  display_name: '',
  category: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: FsmStateDictListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.name) params.name = query.name
    if (query.display_name) params.display_name = query.display_name
    if (query.category) params.category = query.category
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await fsmStateDictApi.list(params)
    tableData.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

async function loadCategoryOptions() {
  try {
    const res = await dictApi.list('fsm_state_category')
    categoryOptions.value = res.data?.items ?? []
  } catch {
    // 非关键，静默失败
  }
}

onMounted(() => {
  fetchList()
  loadCategoryOptions()
})

// ---------- 筛选 ----------

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.name = ''
  query.display_name = ''
  query.category = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: FsmStateDictListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用状态「${row.display_name}」（${row.name}）？`
    : `确认禁用状态「${row.display_name}」（${row.name}）？禁用后仍可被已有 FSM 配置引用，但新引用需重新启用。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    const detail = await fsmStateDictApi.detail(row.id)
    await fsmStateDictApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === FSM_STATE_DICT_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: FsmStateDictListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'edit',
      entityType: 'fsm-state-dict',
      entity: { id: row.id, name: row.name, label: row.display_name },
    })
    return
  }
  router.push(`/fsm-state-dicts/${row.id}/edit`)
}

async function handleDelete(row: FsmStateDictListItem) {
  if (row.enabled) {
    guardRef.value?.open({
      action: 'delete',
      entityType: 'fsm-state-dict',
      entity: { id: row.id, name: row.name, label: row.display_name },
    })
    return
  }
  // 已禁用：先查引用，有引用弹详情阻止，无引用确认删除
  try {
    const res = await fsmStateDictApi.references(row.id)
    const configs = res.data?.fsm_configs || []
    if (configs.length > 0) {
      showRefDialog(row, configs)
      ElMessage.warning(`该状态被 ${configs.length} 个状态机引用，无法删除。请先移除引用关系。`)
      return
    }
  } catch {
    return
  }
  // 无引用：确认删除
  try {
    await ElMessageBox.confirm(
      `确认删除状态「${row.display_name}」（${row.name}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await fsmStateDictApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    const bizErr = err as BizError
    if (bizErr.code === FSM_STATE_DICT_ERR.IN_USE) {
      // 后端兜底：重新拉引用详情展示
      await loadAndShowRefs(row)
      return
    }
    // 其他错误拦截器已 toast
  }
}

// ---------- 引用弹窗 ----------

function showRefDialog(row: FsmStateDictListItem, configs: FsmStateDictRefConfigItem[]) {
  refDialog.visible = true
  refDialog.loading = false
  refDialog.name = row.name
  refDialog.label = row.display_name
  refDialog.fsmConfigs = configs
}

async function loadAndShowRefs(row: FsmStateDictListItem) {
  refDialog.visible = true
  refDialog.loading = true
  refDialog.name = row.name
  refDialog.label = row.display_name
  refDialog.fsmConfigs = []
  try {
    const res = await fsmStateDictApi.references(row.id)
    refDialog.fsmConfigs = res.data?.fsm_configs || []
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
  refDialog.fsmConfigs = []
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: FsmStateDictListItem }) {
  return row.enabled ? '' : 'row-disabled'
}

</script>
