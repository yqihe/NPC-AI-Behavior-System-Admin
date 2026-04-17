<template>
  <div class="list-root">
    <!-- 顶部标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">模板管理</h2>
      </div>
      <div class="header-right">
        <el-button type="primary" @click="$router.push('/templates/create')">
          <el-icon><Plus /></el-icon>
          新建模板
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
        <el-table-column prop="name" label="模板标识" min-width="160" />
        <el-table-column prop="label" label="中文标签" min-width="160" />
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
            <el-link type="primary" :underline="false" @click="router.push(`/templates/${row.id}/view`)">查看</el-link>
            <el-link type="primary" :underline="false" style="margin-left: 12px" @click="handleEdit(row)">编辑</el-link>
            <el-link type="danger" :underline="false" style="margin-left: 12px" @click="handleDelete(row)">删除</el-link>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无模板数据">
            <el-button type="primary" @click="$router.push('/templates/create')">
              新建模板
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
      width="500px"
      @close="resetRefDialog"
    >
      <div v-loading="refDialog.loading">
        <div class="ref-section">
          <p class="ref-subtitle">
            NPC 引用（{{ refDialog.npcs.length }} 个 NPC 使用了该模板）：
          </p>
          <el-table
            v-if="refDialog.npcs.length > 0"
            :data="refDialog.npcs"
            size="small"
          >
            <el-table-column prop="npc_name" label="NPC 标识" />
          </el-table>
          <p v-else class="ref-empty">暂无 NPC 引用</p>
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
import { templateApi, TEMPLATE_ERR } from '@/api/templates'
import type { TemplateListItem, TemplateListQuery, TemplateReferenceItem } from '@/api/templates'
import type { BizError } from '@/api/request'
import { formatTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const tableData = ref<TemplateListItem[]>([])
const total = ref(0)
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)

const refDialog = reactive({
  visible: false,
  loading: false,
  name: '',
  label: '',
  npcs: [] as TemplateReferenceItem[],
})

const query = reactive<TemplateListQuery>({
  name: '',
  label: '',
  enabled: null,
  page: 1,
  page_size: 20,
})

// ---------- 数据加载 ----------

async function fetchList() {
  loading.value = true
  try {
    const params: TemplateListQuery = {
      page: query.page,
      page_size: query.page_size,
    }
    if (query.name) params.name = query.name
    if (query.label) params.label = query.label
    if (query.enabled !== null && query.enabled !== undefined) {
      params.enabled = query.enabled
    }
    const res = await templateApi.list(params)
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
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: TemplateListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  const msg = val
    ? `确认启用模板「${row.label}」？启用后可被 NPC 管理页选择。`
    : `确认禁用模板「${row.label}」？禁用后 NPC 管理页将无法看到，已有 NPC 不受影响。`
  try {
    await ElMessageBox.confirm(msg, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    // 列表接口不返回 version，先 detail 拿最新 version
    const detail = await templateApi.detail(row.id)
    await templateApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === TEMPLATE_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
    }
    // 其他错误拦截器已 toast
  }
}

function handleEdit(row: TemplateListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'edit', entityType: 'template', entity: row })
    return
  }
  router.push(`/templates/${row.id}/edit`)
}

async function handleDelete(row: TemplateListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'delete', entityType: 'template', entity: row })
    return
  }
  // 已禁用：先查引用，有引用弹详情阻止，无引用确认删除
  try {
    const res = await templateApi.references(row.id)
    const npcs = res.data?.npcs || []
    if (npcs.length > 0) {
      showRefDialog(row, npcs)
      ElMessage.warning(`该模板被 ${npcs.length} 个 NPC 引用，无法删除。请先移除引用关系。`)
      return
    }
  } catch {
    // references API 失败拦截器已 toast；为安全起见不继续删除
    return
  }
  // 无引用：确认删除
  try {
    await ElMessageBox.confirm(
      `确认删除模板「${row.label}」（${row.name}）？删除后无法恢复。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await templateApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    // 后端兜底：REF_DELETE 时重新拉引用详情展示
    if ((err as BizError).code === TEMPLATE_ERR.REF_DELETE) {
      await loadAndShowRefs(row)
    }
    // 其他错误拦截器已 toast
  }
}

function showRefDialog(row: TemplateListItem, npcs: TemplateReferenceItem[]) {
  refDialog.visible = true
  refDialog.loading = false
  refDialog.name = row.name
  refDialog.label = row.label
  refDialog.npcs = npcs
}

async function loadAndShowRefs(row: TemplateListItem) {
  refDialog.visible = true
  refDialog.loading = true
  refDialog.name = row.name
  refDialog.label = row.label
  refDialog.npcs = []
  try {
    const res = await templateApi.references(row.id)
    refDialog.npcs = res.data?.npcs || []
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
  refDialog.npcs = []
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: TemplateListItem }) {
  return row.enabled ? '' : 'row-disabled'
}

</script>

