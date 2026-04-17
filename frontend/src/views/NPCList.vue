<template>
  <div class="list-root">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">NPC 管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/npcs/create')">
          <el-icon><Plus /></el-icon>
          新建 NPC
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
        v-model="query.template_name"
        placeholder="所用模板"
        clearable
        filterable
        class="filter-item"
      >
        <el-option
          v-for="tpl in templateOptions"
          :key="tpl.name"
          :label="`${tpl.label} (${tpl.name})`"
          :value="tpl.name"
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
        <el-table-column prop="name" label="NPC 标识" min-width="160" />
        <el-table-column prop="label" label="中文标签" min-width="140" />
        <el-table-column label="所用模板" width="160">
          <template #default="{ row }">
            <span v-if="row.template_label">{{ row.template_label }}</span>
            <span v-else class="text-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="行为状态机" width="140">
          <template #default="{ row }">
            <span v-if="row.fsm_ref">{{ row.fsm_ref }}</span>
            <span v-else class="text-muted">—</span>
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
            <el-link type="primary" :underline="false" @click="router.push(`/npcs/${row.id}/view`)">查看</el-link>
            <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
            <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无 NPC 数据">
            <el-button type="primary" @click="$router.push('/npcs/create')">
              新建 NPC
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import { npcApi, NPC_ERRORS } from '@/api/npc'
import type { NPCListItem, NPCListQuery } from '@/api/npc'
import { templateApi } from '@/api/templates'
import type { TemplateListItem } from '@/api/templates'
import type { BizError } from '@/api/request'
import { formatTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const tableData = ref<NPCListItem[]>([])
const total = ref(0)
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)
const templateOptions = ref<TemplateListItem[]>([])

const query = reactive<NPCListQuery>({
  name: '',
  label: '',
  template_name: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: NPCListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.name) params.name = query.name
    if (query.label) params.label = query.label
    if (query.template_name) params.template_name = query.template_name
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await npcApi.list(params)
    tableData.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  const tplRes = await templateApi.list({ page: 1, page_size: 1000 })
  templateOptions.value = tplRes.data?.items || []
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
  query.template_name = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: NPCListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用 NPC「${row.label}」？`
    : `确认禁用 NPC「${row.label}」？禁用后导出 API 将不包含此 NPC。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    // 列表接口不返回 version，先 detail 拿最新 version
    const detail = await npcApi.detail(row.id)
    await npcApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === NPC_ERRORS.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: NPCListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'edit', entityType: 'npc', entity: row })
    return
  }
  router.push(`/npcs/${row.id}/edit`)
}

async function handleDelete(row: NPCListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'delete', entityType: 'npc', entity: row })
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除 NPC「${row.label}」（${row.name}）？删除后无法恢复，NPC 标识也不可再复用。`,
      '删除确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
    await npcApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    if ((err as BizError).code === NPC_ERRORS.DELETE_NOT_DISABLED) {
      // 兜底：删除时后端仍返回 45013（启用中）
      guardRef.value?.open({ action: 'delete', entityType: 'npc', entity: row })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: NPCListItem }) {
  return row.enabled ? '' : 'row-disabled'
}
</script>
