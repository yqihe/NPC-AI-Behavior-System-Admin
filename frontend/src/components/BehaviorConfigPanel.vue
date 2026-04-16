<template>
  <div class="behavior-config-panel">
    <!-- FSM 选择区 -->
    <div class="panel-row">
      <span class="panel-label">行为状态机</span>
      <div class="panel-field">
        <template v-if="fsmList.length === 0 && !disabled">
          <el-alert type="warning" :closable="false" show-icon>
            暂无可用的状态机。
            <el-link type="primary" :underline="false" style="margin-left: 4px" @click="$router.push('/fsm-configs')">
              去状态机管理
            </el-link>
          </el-alert>
        </template>
        <template v-else>
          <el-select
            :model-value="modelValue.fsm_ref"
            placeholder="选择状态机（可选）"
            clearable
            :disabled="disabled"
            style="width: 100%"
            @change="onFsmChange"
          >
            <el-option
              v-for="fsm in fsmList"
              :key="fsm.name"
              :label="`${fsm.display_name} (${fsm.name})`"
              :value="fsm.name"
            />
          </el-select>
        </template>
      </div>
    </div>

    <!-- BT 引用动态表（FSM 选中后显示） -->
    <template v-if="modelValue.fsm_ref">
      <div class="bt-table-header panel-row">
        <span class="panel-label">行为树绑定</span>
        <span class="bt-table-hint">每个 FSM 状态可选绑定一棵行为树</span>
      </div>

      <div v-if="btList.length === 0 && !disabled" class="panel-row">
        <span class="panel-label"></span>
        <el-alert type="warning" :closable="false" show-icon style="flex: 1">
          暂无可用的行为树。
          <el-link type="primary" :underline="false" style="margin-left: 4px" @click="$router.push('/bt-trees')">
            去行为树管理
          </el-link>
        </el-alert>
      </div>

      <template v-else>
        <div v-if="fsmStates.length === 0" class="panel-row">
          <span class="panel-label"></span>
          <span class="text-muted">加载状态列表中...</span>
        </div>
        <div
          v-for="stateName in fsmStates"
          :key="stateName"
          class="panel-row bt-row"
        >
          <span class="panel-label state-label">
            <span class="state-name">{{ stateName }}</span>
            <span class="state-tag">FSM 状态</span>
          </span>
          <el-select
            :model-value="modelValue.bt_refs[stateName] ?? ''"
            placeholder="不绑定行为树"
            clearable
            :disabled="disabled"
            style="width: 100%"
            @change="(val: string) => onBtChange(stateName, val)"
          >
            <el-option
              v-for="bt in btList"
              :key="bt.name"
              :label="`${bt.display_name} (${bt.name})`"
              :value="bt.name"
            />
          </el-select>
        </div>
      </template>
    </template>

    <!-- 查看模式：无 FSM 时显示"未配置行为" -->
    <template v-else-if="disabled">
      <div class="panel-row">
        <span class="panel-label"></span>
        <span class="text-muted">未配置行为</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { FsmConfigListItem } from '@/api/fsmConfigs'
import type { BtTreeListItem } from '@/api/btTrees'

const router = useRouter()

export interface BehaviorConfig {
  fsm_ref: string
  bt_refs: Record<string, string>
}

const props = defineProps<{
  modelValue: BehaviorConfig
  disabled: boolean
  fsmList: FsmConfigListItem[]
  btList: BtTreeListItem[]
  /** FSM 选中后由父组件注入的 state 名称列表 */
  fsmStates: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: BehaviorConfig]
}>()

function onFsmChange(newFsmRef: string | undefined) {
  // FSM 变更时清空 bt_refs
  emit('update:modelValue', {
    fsm_ref: newFsmRef ?? '',
    bt_refs: {},
  })
}

function onBtChange(stateName: string, btName: string) {
  const newBtRefs = { ...props.modelValue.bt_refs }
  if (btName) {
    newBtRefs[stateName] = btName
  } else {
    delete newBtRefs[stateName]
  }
  emit('update:modelValue', {
    fsm_ref: props.modelValue.fsm_ref,
    bt_refs: newBtRefs,
  })
}
</script>

<style scoped>
.behavior-config-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.panel-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.panel-label {
  width: 120px;
  flex-shrink: 0;
  font-size: 14px;
  color: #606266;
  text-align: right;
}

.panel-field {
  flex: 1;
}

.bt-table-header {
  margin-top: 4px;
}

.bt-table-hint {
  font-size: 12px;
  color: #909399;
}

.bt-row {
  align-items: center;
}

.state-label {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  line-height: 1.2;
}

.state-name {
  font-family: 'Courier New', Courier, monospace;
  font-size: 13px;
  color: #303133;
}

.state-tag {
  font-size: 11px;
  color: #909399;
}

.text-muted {
  color: #C0C4CC;
  font-size: 14px;
}
</style>
