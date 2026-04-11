<template>
  <el-dialog
    v-model="visible"
    :title="title"
    width="480px"
    :close-on-click-modal="false"
    @close="onClose"
  >
    <div class="guard-content">
      <div class="guard-header">
        <el-icon class="warn-icon"><WarningFilled /></el-icon>
        <div class="guard-title">
          <h3>{{ title }}</h3>
          <p class="guard-template">{{ template?.label }} ({{ template?.name }})</p>
        </div>
      </div>
      <p class="guard-reason">{{ reasonText }}</p>
      <div v-if="action === 'edit'" class="guard-steps">
        <div class="steps-label">操作步骤</div>
        <ol>
          <li>在列表中点击该模板的「启用」开关停用它</li>
          <li>完成编辑后再次启用</li>
        </ol>
      </div>
      <div v-else class="guard-conditions">
        <div class="steps-label">删除前置条件</div>
        <ul class="conditions-list">
          <li class="cond-fail">
            <el-icon><CircleCloseFilled /></el-icon>
            <span>模板已停用</span>
          </li>
          <li :class="template && template.ref_count === 0 ? 'cond-pass' : 'cond-fail'">
            <el-icon v-if="template && template.ref_count === 0"><CircleCheckFilled /></el-icon>
            <el-icon v-else><CircleCloseFilled /></el-icon>
            <span>
              没有 NPC 在使用该模板（当前被引用：{{ template?.ref_count ?? 0 }}）
            </span>
          </li>
        </ul>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">知道了</el-button>
      <el-button type="warning" :loading="acting" @click="onActOnce">
        立即停用
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  WarningFilled,
  CircleCheckFilled,
  CircleCloseFilled,
} from '@element-plus/icons-vue'
import { templateApi, TEMPLATE_ERR } from '@/api/templates'
import type { TemplateListItem } from '@/api/templates'
import type { BizError } from '@/api/request'

type GuardAction = 'edit' | 'delete'

const router = useRouter()
const visible = ref(false)
const acting = ref(false)
const action = ref<GuardAction>('edit')
const template = ref<TemplateListItem | null>(null)

const title = computed(() =>
  action.value === 'edit' ? '无法编辑模板' : '无法删除模板',
)

const reasonText = computed(() =>
  action.value === 'edit'
    ? '启用中模板对 NPC 管理页可见，允许任意修改可能导致策划在配置不稳定时选用。请先停用该模板后再进行编辑。'
    : '删除是不可恢复的操作，先停用可以提供一个观察期，确认无误后再删除。',
)

const emit = defineEmits<{
  refresh: []
}>()

function open(act: GuardAction, tpl: TemplateListItem) {
  action.value = act
  template.value = tpl
  visible.value = true
}

function onClose() {
  template.value = null
  acting.value = false
}

async function onActOnce() {
  if (!template.value) return
  acting.value = true
  try {
    // 列表接口不返回 version，先 detail 拿最新 version
    const detailRes = await templateApi.detail(template.value.id)
    const version = detailRes.data.version
    await templateApi.toggleEnabled(template.value.id, false, version)
    ElMessage.success('已停用')

    if (action.value === 'edit') {
      const id = template.value.id
      visible.value = false
      router.push(`/templates/${id}/edit`)
    } else {
      // 删除场景：不自动触发删除，请父组件刷新列表让用户再点一次「删除」
      visible.value = false
      emit('refresh')
    }
  } catch (err) {
    const bizErr = err as BizError
    if (bizErr.code === TEMPLATE_ERR.VERSION_CONFLICT) {
      ElMessage.warning('该模板已被其他人修改，请刷新后重试')
      emit('refresh')
      visible.value = false
    }
    // 其他错误拦截器已 toast
  } finally {
    acting.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
.guard-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.guard-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.warn-icon {
  color: #E6A23C;
  font-size: 28px;
  flex-shrink: 0;
  margin-top: 2px;
}

.guard-title h3 {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 4px 0;
}

.guard-template {
  font-size: 12px;
  color: #909399;
  margin: 0;
}

.guard-reason {
  font-size: 13px;
  color: #606266;
  line-height: 1.6;
  margin: 0;
  padding-left: 40px;
}

.guard-steps,
.guard-conditions {
  background: #FDF6EC;
  border-left: 3px solid #E6A23C;
  border-radius: 4px;
  padding: 12px 14px;
  margin-left: 40px;
}

.steps-label {
  font-size: 12px;
  font-weight: 600;
  color: #E6A23C;
  margin-bottom: 6px;
}

.guard-steps ol {
  margin: 0;
  padding-left: 18px;
  font-size: 13px;
  color: #606266;
  line-height: 1.8;
}

.conditions-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.conditions-list li {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.cond-fail {
  color: #F56C6C;
}

.cond-pass {
  color: #67C23A;
}
</style>
