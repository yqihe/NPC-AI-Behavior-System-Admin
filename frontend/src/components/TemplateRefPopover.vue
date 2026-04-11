<template>
  <el-dialog
    v-model="visible"
    :title="dialogTitle"
    width="560px"
    :close-on-click-modal="false"
    @close="onClose"
  >
    <div class="pop-body">
      <div class="pop-info">
        <el-icon><InfoFilled /></el-icon>
        <span>勾选的子字段会扁平地写入模板，与其他来源自动去重</span>
      </div>

      <div v-if="loading" class="pop-loading">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span>加载中...</span>
      </div>

      <template v-else>
        <div class="pop-toolbar">
          <span class="pop-count-left">子字段 ({{ subFields.length }})</span>
          <el-button v-if="!readonly" text type="primary" @click="selectAll">
            <el-icon><Check /></el-icon>全选
          </el-button>
          <el-button v-if="!readonly" text @click="deselectAll">
            <el-icon><Close /></el-icon>全不选
          </el-button>
        </div>

        <div v-if="subFields.length === 0" class="pop-empty">
          该 reference 字段尚未配置任何子字段
        </div>
        <div v-else class="pop-list">
          <label
            v-for="f in subFields"
            :key="f.id"
            class="pop-item"
            :class="{ readonly }"
          >
            <el-checkbox
              :model-value="tempSelected.includes(f.id)"
              :disabled="readonly"
              @change="(v: string | number | boolean) => toggle(f.id, Boolean(v))"
            />
            <span class="pop-label">{{ f.label }}</span>
            <span class="pop-name">{{ f.name }}</span>
            <el-tag size="small" type="info">{{ f.type_label || f.type }}</el-tag>
          </label>
        </div>
      </template>
    </div>

    <template #footer>
      <span class="pop-count-footer">
        已选 {{ tempSelected.length }} / {{ subFields.length }}
      </span>
      <el-button @click="visible = false">
        {{ readonly ? '关闭' : '取消' }}
      </el-button>
      <el-button v-if="!readonly" type="primary" @click="onConfirm">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { InfoFilled, Loading, Check, Close } from '@element-plus/icons-vue'
import { fieldApi } from '@/api/fields'
import type { FieldListItem } from '@/api/fields'

/** reference 字段 properties.constraints.ref_fields 里的富对象 */
interface RefFieldItem {
  id: number
  name: string
  label: string
  type: string
  type_label?: string
}

defineProps<{
  readonly?: boolean
}>()

const emit = defineEmits<{
  confirm: [payload: { allSubIds: number[]; selectedSubIds: number[] }]
}>()

const visible = ref(false)
const loading = ref(false)
const subFields = ref<RefFieldItem[]>([])
const tempSelected = ref<number[]>([])
const refFieldLabel = ref('')
const refFieldName = ref('')

const dialogTitle = computed(() =>
  refFieldLabel.value
    ? `${refFieldLabel.value} — 选择子字段`
    : '选择子字段',
)

async function open(refField: FieldListItem, currentSelectedIds: number[]) {
  visible.value = true
  loading.value = true
  subFields.value = []
  tempSelected.value = []
  refFieldLabel.value = refField.label
  refFieldName.value = refField.name
  try {
    // 后端持久化格式是 constraints.refs (number[])，不是富对象 ref_fields
    // FieldForm.vue 只在 UI 本地状态下用 ref_fields，提交前转回 refs
    const res = await fieldApi.detail(refField.id)
    const constraints = (res.data?.properties?.constraints ?? {}) as {
      refs?: number[]
    }
    const refIds = constraints.refs ?? []
    if (refIds.length === 0) {
      subFields.value = []
      return
    }
    // 并发拉每个子字段详情（reference 禁止嵌套，子字段必是 leaf，数量通常 < 10）
    const details = await Promise.all(
      refIds.map((id) =>
        fieldApi
          .detail(id)
          .then((r) => r.data)
          .catch(() => null),
      ),
    )
    const items: RefFieldItem[] = []
    for (let i = 0; i < refIds.length; i++) {
      const d = details[i]
      if (d) {
        items.push({
          id: d.id,
          name: d.name,
          label: d.label,
          type: d.type,
        })
      } else {
        // 罕见：子字段被硬删除，保留占位以便视觉呈现
        items.push({
          id: refIds[i],
          name: `field_${refIds[i]}`,
          label: `字段 ${refIds[i]}`,
          type: 'unknown',
        })
      }
    }
    subFields.value = items
    // 回勾：currentSelectedIds 与 subFields 的交集
    const subIdSet = new Set(items.map((f) => f.id))
    tempSelected.value = currentSelectedIds.filter((id) => subIdSet.has(id))
  } catch {
    // 拦截器已 toast
    visible.value = false
  } finally {
    loading.value = false
  }
}

function toggle(id: number, checked: boolean) {
  if (checked) {
    if (!tempSelected.value.includes(id)) {
      tempSelected.value = [...tempSelected.value, id]
    }
  } else {
    tempSelected.value = tempSelected.value.filter((x) => x !== id)
  }
}

function selectAll() {
  tempSelected.value = subFields.value.map((f) => f.id)
}

function deselectAll() {
  tempSelected.value = []
}

function onConfirm() {
  emit('confirm', {
    allSubIds: subFields.value.map((f) => f.id),
    selectedSubIds: tempSelected.value.slice(),
  })
  visible.value = false
}

function onClose() {
  subFields.value = []
  tempSelected.value = []
  refFieldLabel.value = ''
  refFieldName.value = ''
}

defineExpose({ open })
</script>

<style scoped>
.pop-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.pop-info {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: #ecf5ff;
  border-radius: 4px;
  font-size: 12px;
  color: #409eff;
}

.pop-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 0;
  color: #909399;
  font-size: 13px;
}

.pop-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding-bottom: 4px;
  border-bottom: 1px solid #ebeef5;
}

.pop-count-left {
  flex: 1;
  font-size: 12px;
  color: #606266;
  font-weight: 600;
}

.pop-empty {
  text-align: center;
  color: #c0c4cc;
  font-size: 13px;
  padding: 24px 0;
}

.pop-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 320px;
  overflow-y: auto;
}

.pop-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 4px;
  cursor: pointer;
}

.pop-item:hover {
  background: #f5f7fa;
}

.pop-item.readonly {
  cursor: default;
}

.pop-label {
  font-size: 13px;
  color: #303133;
  font-weight: 500;
}

.pop-name {
  font-size: 12px;
  color: #909399;
  flex: 1;
}

.pop-count-footer {
  margin-right: auto;
  font-size: 12px;
  color: #606266;
}
</style>
