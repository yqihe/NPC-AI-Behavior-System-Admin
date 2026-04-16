<template>
  <el-dialog
    v-model="visible"
    :show-close="true"
    width="480px"
    :close-on-click-modal="false"
    append-to-body
    @close="onClose"
  >
    <template #header>
      <div class="guard-header">
        <div class="guard-header-icon">
          <el-icon><WarningFilled /></el-icon>
        </div>
        <span class="guard-header-title">{{ title }}</span>
      </div>
    </template>

    <div class="guard-body">
      <p class="guard-lead">{{ leadText }}</p>
      <p class="guard-reason">{{ reasonText }}</p>

      <!-- 编辑：操作步骤 -->
      <div v-if="action === 'edit'" class="guard-box">
        <div class="guard-box-label">操作步骤</div>
        <div class="guard-box-line">1. 在列表中点击该{{ entityTypeLabel }}的「启用」开关禁用它</div>
        <div class="guard-box-line">2. 完成编辑后再次启用</div>
      </div>

      <!-- 删除：前置条件 -->
      <div v-else class="guard-box">
        <div class="guard-box-label">删除前置条件</div>
        <div class="guard-cond guard-cond-fail">
          <el-icon><CircleCloseFilled /></el-icon>
          <span>{{ entityTypeLabel }}已禁用</span>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="guard-footer">
        <el-button @click="visible = false">知道了</el-button>
        <el-button type="warning" :loading="acting" @click="onActOnce">
          <el-icon v-if="!acting" class="btn-icon"><SwitchButton /></el-icon>
          立即禁用
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  WarningFilled,
  CircleCloseFilled,
  SwitchButton,
} from '@element-plus/icons-vue'
import { fieldApi, FIELD_ERR } from '@/api/fields'
import { templateApi, TEMPLATE_ERR } from '@/api/templates'
import { eventTypeApi, EVENT_TYPE_ERR, EXT_SCHEMA_ERR } from '@/api/eventTypes'
import { fsmStateDictApi, FSM_STATE_DICT_ERR } from '@/api/fsmStateDicts'
import { fsmConfigApi, FSM_ERR } from '@/api/fsmConfigs'
import { btTreeApi, BT_TREE_ERR } from '@/api/btTrees'
import { btNodeTypeApi, BT_NODE_TYPE_ERR } from '@/api/btNodeTypes'
import { npcApi, NPC_ERRORS } from '@/api/npc'
import type { BizError } from '@/api/request'

type GuardAction = 'edit' | 'delete'
type EntityType = 'field' | 'template' | 'event-type' | 'event-type-schema' | 'fsm-state-dict' | 'fsm-config' | 'bt-tree' | 'bt-node-type' | 'npc'

interface GuardEntity {
  id: number
  name: string
  label: string
}

const router = useRouter()
const visible = ref(false)
const acting = ref(false)
const action = ref<GuardAction>('edit')
const entityType = ref<EntityType>('template')
const entity = ref<GuardEntity | null>(null)

const entityTypeLabel = computed(() => {
  if (entityType.value === 'field') return '字段'
  if (entityType.value === 'event-type') return '事件类型'
  if (entityType.value === 'event-type-schema') return '扩展字段'
  if (entityType.value === 'fsm-state-dict') return '状态字典'
  if (entityType.value === 'fsm-config') return '状态机'
  if (entityType.value === 'bt-tree') return '行为树'
  if (entityType.value === 'bt-node-type') return '节点类型'
  if (entityType.value === 'npc') return 'NPC'
  return '模板'
})

const title = computed(() =>
  action.value === 'edit'
    ? `无法编辑${entityTypeLabel.value}`
    : `无法删除${entityTypeLabel.value}`,
)

const leadText = computed(() => {
  const verb = action.value === 'edit' ? '编辑' : '删除'
  return `该${entityTypeLabel.value}当前处于启用状态，无法直接${verb}。`
})

const reasonText = computed(() => {
  if (action.value === 'edit') {
    if (entityType.value === 'field') {
      return '已启用的字段对模板与其他字段可见，允许任意修改可能导致引用方看到不稳定的配置。请先禁用，再进入编辑。'
    }
    if (entityType.value === 'event-type') {
      return '已启用的事件类型对 FSM/BT 可见，任意修改可能导致引用方看到不稳定的配置。请先禁用，再进入编辑。'
    }
    if (entityType.value === 'event-type-schema') {
      return '已启用的扩展字段对事件类型表单可见，任意修改可能导致表单配置不稳定。请先禁用，再进入编辑。'
    }
    if (entityType.value === 'fsm-config') {
      return '已启用的状态机对游戏服务端可见，任意修改可能导致服务端拉取到不稳定配置。请先禁用，再进入编辑。'
    }
    if (entityType.value === 'bt-tree') {
      return '已启用的行为树对游戏服务端可见，任意修改可能导致服务端拉取到不稳定配置。请先禁用，再进入编辑。'
    }
    if (entityType.value === 'bt-node-type') {
      return '已启用的节点类型被树编辑器使用，修改参数定义可能导致已有行为树节点渲染异常。请先禁用，再进入编辑。'
    }
    if (entityType.value === 'npc') {
      return '已启用的 NPC 会被游戏服务端导出接口返回，修改期间可能导致服务端拉取到不稳定配置。请先禁用，再进入编辑。'
    }
    return '已启用的模板对 NPC 管理页可见，允许任意修改可能导致策划在配置不稳定时选用。请先禁用，再进入编辑。'
  }
  return '删除是不可恢复的操作。先禁用可以提供一个观察期 — 确认下线没有问题，再执行删除。'
})

const emit = defineEmits<{
  refresh: []
}>()

function open(opts: {
  action: GuardAction
  entityType: EntityType
  entity: GuardEntity
}) {
  action.value = opts.action
  entityType.value = opts.entityType
  entity.value = opts.entity
  visible.value = true
}

function onClose() {
  entity.value = null
  acting.value = false
}

async function onActOnce() {
  if (!entity.value) return
  acting.value = true
  const id = entity.value.id
  try {
    if (entityType.value === 'field') {
      const detail = await fieldApi.detail(id)
      await fieldApi.toggleEnabled(id, false, detail.data.version)
    } else if (entityType.value === 'event-type') {
      const detail = await eventTypeApi.detail(id)
      await eventTypeApi.toggleEnabled(id, false, detail.data.version)
    } else if (entityType.value === 'event-type-schema') {
      // Schema 无 detail 接口，用 list 查找目标项获取 version
      const listRes = await eventTypeApi.schemaList()
      const target = (listRes.data?.items || []).find((s) => s.id === id)
      if (!target) {
        ElMessage.error('扩展字段不存在')
        visible.value = false
        return
      }
      await eventTypeApi.schemaToggleEnabled(id, false, target.version)
    } else if (entityType.value === 'fsm-state-dict') {
      const detail = await fsmStateDictApi.detail(id)
      await fsmStateDictApi.toggleEnabled(id, false, detail.data.version)
    } else if (entityType.value === 'fsm-config') {
      const detail = await fsmConfigApi.detail(id)
      await fsmConfigApi.toggleEnabled(id, false, detail.data.version)
    } else if (entityType.value === 'bt-tree') {
      const detail = await btTreeApi.detail(id)
      await btTreeApi.toggleEnabled(id, false, detail.data.version)
    } else if (entityType.value === 'bt-node-type') {
      const detail = await btNodeTypeApi.detail(id)
      await btNodeTypeApi.toggleEnabled(id, false, detail.data.version)
    } else if (entityType.value === 'npc') {
      const detail = await npcApi.detail(id)
      await npcApi.toggleEnabled(id, false, detail.data.version)
    } else {
      const detail = await templateApi.detail(id)
      await templateApi.toggleEnabled(id, false, detail.data.version)
    }
    ElMessage.success('已禁用')

    if (action.value === 'edit') {
      visible.value = false
      let path: string
      if (entityType.value === 'field') {
        path = `/fields/${id}/edit`
      } else if (entityType.value === 'event-type') {
        path = `/event-types/${id}/edit`
      } else if (entityType.value === 'event-type-schema') {
        path = `/event-type-schemas/${id}/edit`
      } else if (entityType.value === 'fsm-state-dict') {
        path = `/fsm-state-dicts/${id}/edit`
      } else if (entityType.value === 'fsm-config') {
        path = `/fsm-configs/${id}/edit`
      } else if (entityType.value === 'bt-tree') {
        path = `/bt-trees/${id}/edit`
      } else if (entityType.value === 'bt-node-type') {
        path = `/bt-node-types/${id}/edit`
      } else if (entityType.value === 'npc') {
        path = `/npcs/${id}/edit`
      } else {
        path = `/templates/${id}/edit`
      }
      router.push(path)
    } else {
      // 删除场景：不自动触发删除，请父组件刷新列表让用户再点一次「删除」
      visible.value = false
      emit('refresh')
    }
  } catch (err) {
    const bizErr = err as BizError
    let conflictCode: number
    if (entityType.value === 'field') {
      conflictCode = FIELD_ERR.VERSION_CONFLICT
    } else if (entityType.value === 'event-type') {
      conflictCode = EVENT_TYPE_ERR.VERSION_CONFLICT
    } else if (entityType.value === 'event-type-schema') {
      conflictCode = EXT_SCHEMA_ERR.VERSION_CONFLICT
    } else if (entityType.value === 'fsm-state-dict') {
      conflictCode = FSM_STATE_DICT_ERR.VERSION_CONFLICT
    } else if (entityType.value === 'fsm-config') {
      conflictCode = FSM_ERR.VERSION_CONFLICT
    } else if (entityType.value === 'bt-tree') {
      conflictCode = BT_TREE_ERR.VERSION_CONFLICT
    } else if (entityType.value === 'bt-node-type') {
      conflictCode = BT_NODE_TYPE_ERR.VERSION_CONFLICT
    } else if (entityType.value === 'npc') {
      conflictCode = NPC_ERRORS.VERSION_CONFLICT
    } else {
      conflictCode = TEMPLATE_ERR.VERSION_CONFLICT
    }
    if (bizErr.code === conflictCode) {
      ElMessage.warning(`该${entityTypeLabel.value}已被其他人修改，请刷新后重试`)
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
.guard-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.guard-header-icon {
  width: 24px;
  height: 24px;
  border-radius: 12px;
  background: #FDF6EC;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #E6A23C;
  font-size: 14px;
  flex-shrink: 0;
}

.guard-header-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.guard-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.guard-lead {
  font-size: 14px;
  font-weight: 500;
  color: #303133;
  margin: 0;
  line-height: 1.6;
}

.guard-reason {
  font-size: 13px;
  color: #606266;
  line-height: 1.6;
  margin: 0;
}

.guard-box {
  background: #F5F7FA;
  border: 1px solid #E4E7ED;
  border-radius: 4px;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.guard-box-label {
  font-size: 12px;
  font-weight: 500;
  color: #909399;
  margin-bottom: 2px;
}

.guard-box-line {
  font-size: 12px;
  color: #606266;
  line-height: 1.6;
}

.guard-cond {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  line-height: 1.6;
}

.guard-cond .el-icon {
  font-size: 14px;
  flex-shrink: 0;
}

.guard-cond-fail {
  color: #F56C6C;
}

.guard-cond-pass {
  color: #67C23A;
}

.guard-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.btn-icon {
  margin-right: 4px;
}
</style>
