<template>
  <div class="template-list">
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
        v-model="query.label"
        placeholder="搜索中文标签"
        clearable
        class="filter-item filter-item-wide"
        @keyup.enter="handleSearch"
      />
      <el-select
        v-model="query.enabled"
        placeholder="启用状态"
        clearable
        class="filter-item"
      >
        <el-option label="已启用" :value="true" />
        <el-option label="已停用" :value="false" />
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
        <el-table-column label="被引用数" width="100" align="center">
          <template #default="{ row }: { row: TemplateListItem }">
            <el-link
              type="primary"
              :underline="false"
              @click="handleShowRefs(row)"
            >
              {{ row.ref_count }}
            </el-link>
          </template>
        </el-table-column>
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }: { row: TemplateListItem }">
            <el-switch
              :model-value="row.enabled"
              @change="(val: string | number | boolean) => handleToggle(row, Boolean(val))"
            />
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }: { row: TemplateListItem }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }: { row: TemplateListItem }">
            <el-link
              type="primary"
              :underline="false"
              @click="handleEdit(row)"
            >
              编辑
            </el-link>
            <el-link
              type="danger"
              :underline="false"
              style="margin-left: 12px"
              @click="handleDelete(row)"
            >
              删除
            </el-link>
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
    <TemplateReferencesDialog ref="refsRef" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import EnabledGuardDialog from '@/components/EnabledGuardDialog.vue'
import TemplateReferencesDialog from '@/components/TemplateReferencesDialog.vue'
import { templateApi, TEMPLATE_ERR } from '@/api/templates'
import type { TemplateListItem, TemplateListQuery } from '@/api/templates'
import type { BizError } from '@/api/request'

const router = useRouter()

const loading = ref(false)
const tableData = ref<TemplateListItem[]>([])
const total = ref(0)
const guardRef = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)
const refsRef = ref<InstanceType<typeof TemplateReferencesDialog> | null>(null)

const query = reactive<TemplateListQuery>({
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
  query.label = ''
  query.enabled = null
  query.page = 1
  fetchList()
}

// ---------- 行操作 ----------

async function handleToggle(row: TemplateListItem, val: boolean) {
  const action = val ? '启用' : '停用'
  const msg = val
    ? `确认启用模板「${row.label}」？启用后可被 NPC 管理页选择。`
    : `确认停用模板「${row.label}」？停用后 NPC 管理页将无法看到，已有 NPC 不受影响。`
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
      ElMessageBox.alert(
        '该模板已被其他人修改，请刷新后重试。',
        '版本冲突',
        { type: 'warning' },
      )
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
  if (row.ref_count > 0) {
    // 有 NPC 引用：前端展示引用详情，提示先移除引用
    refsRef.value?.open(row)
    ElMessage.warning(
      `该模板被 ${row.ref_count} 个 NPC 引用，无法删除。请先移除引用关系。`,
    )
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除模板「${row.label}」（${row.name}）？删除后无法恢复，模板标识也不可再复用。`,
      '删除确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
    await templateApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    const bizErr = err as BizError
    if (bizErr.code === TEMPLATE_ERR.REF_DELETE) {
      // 后端兜底：被 NPC 引用，自动打开引用详情
      refsRef.value?.open(row)
      return
    }
    // 其他错误拦截器已 toast
  }
}

function handleShowRefs(row: TemplateListItem) {
  refsRef.value?.open(row)
}

// ---------- 辅助 ----------

function rowClassName({ row }: { row: TemplateListItem }) {
  return row.enabled ? '' : 'row-disabled'
}

function formatTime(str: string) {
  if (!str) return ''
  const d = new Date(str)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.template-list {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background: #fff;
  border-bottom: 1px solid #E4E7ED;
}

.page-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
  background: #fff;
  flex-wrap: wrap;
}

.filter-item {
  flex: 1;
  min-width: 0;
}

.filter-item-wide {
  flex: 1.5;
}

.table-wrap {
  flex: 1;
  padding: 0 24px;
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.pagination-wrap {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
}

.total-text {
  font-size: 13px;
  color: #909399;
}

/* 停用模板整行 opacity 0.5，但操作列（最后一列）保持高亮 */
:deep(.row-disabled td:not(:last-child)) {
  opacity: 0.5;
}
</style>
