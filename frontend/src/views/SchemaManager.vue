<template>
  <div class="schema-manager">
    <h2>Schema 管理</h2>
    <p style="color: #909399; margin-bottom: 16px">
      以下 Schema 由游戏服务端定义，ADMIN 平台只读展示。修改 Schema 请联系服务端开发。
    </p>

    <el-tabs v-model="activeTab" v-loading="loading">
      <!-- 组件 Schema -->
      <el-tab-pane label="组件 Schema" name="components">
        <el-collapse>
          <el-collapse-item
            v-for="item in componentSchemas"
            :key="item.name"
            :name="item.name"
          >
            <template #title>
              <span class="schema-title">
                {{ item.config?.display_name || item.name }}
                <el-tag size="small" type="info" style="margin-left: 8px">{{ item.name }}</el-tag>
              </span>
            </template>
            <div class="schema-meta" v-if="item.config?.blackboard_keys?.length">
              <strong>黑板 Key：</strong>
              <el-tag
                v-for="key in item.config.blackboard_keys"
                :key="key"
                size="small"
                style="margin: 2px"
              >{{ key }}</el-tag>
            </div>
            <pre class="schema-json">{{ formatJson(item.config?.schema) }}</pre>
          </el-collapse-item>
        </el-collapse>
        <el-empty v-if="componentSchemas.length === 0" description="暂无组件 Schema" />
      </el-tab-pane>

      <!-- NPC 预设 -->
      <el-tab-pane label="NPC 预设" name="presets">
        <el-collapse>
          <el-collapse-item
            v-for="item in presets"
            :key="item.name"
            :name="item.name"
          >
            <template #title>
              <span class="schema-title">
                {{ item.config?.display_name || item.name }}
                <el-tag size="small" type="info" style="margin-left: 8px">{{ item.name }}</el-tag>
              </span>
            </template>
            <div class="schema-meta">
              <p><strong>必选组件：</strong>{{ (item.config?.required_components || []).join(', ') || '无' }}</p>
              <p><strong>默认组件：</strong>{{ (item.config?.default_components || []).join(', ') || '无' }}</p>
              <p><strong>可选组件：</strong>{{ (item.config?.optional_components || []).join(', ') || '无' }}</p>
            </div>
            <p v-if="item.config?.description" style="color: #909399">{{ item.config.description }}</p>
          </el-collapse-item>
        </el-collapse>
        <el-empty v-if="presets.length === 0" description="暂无 NPC 预设" />
      </el-tab-pane>

      <!-- BT 节点类型 -->
      <el-tab-pane label="BT 节点类型" name="nodeTypes">
        <el-collapse>
          <el-collapse-item
            v-for="item in nodeTypes"
            :key="item.name"
            :name="item.name"
          >
            <template #title>
              <span class="schema-title">
                {{ item.config?.display_name || item.name }}
                <el-tag size="small" :type="categoryColor(item.config?.category)" style="margin-left: 8px">
                  {{ item.config?.category || 'unknown' }}
                </el-tag>
              </span>
            </template>
            <pre class="schema-json">{{ formatJson(item.config?.params_schema) }}</pre>
          </el-collapse-item>
        </el-collapse>
        <el-empty v-if="nodeTypes.length === 0" description="暂无节点类型" />
      </el-tab-pane>

      <!-- FSM 条件类型 -->
      <el-tab-pane label="FSM 条件类型" name="conditionTypes">
        <el-collapse>
          <el-collapse-item
            v-for="item in conditionTypes"
            :key="item.name"
            :name="item.name"
          >
            <template #title>
              <span class="schema-title">
                {{ item.config?.display_name || item.name }}
                <el-tag size="small" type="info" style="margin-left: 8px">{{ item.name }}</el-tag>
              </span>
            </template>
            <pre class="schema-json">{{ formatJson(item.config?.params_schema) }}</pre>
          </el-collapse-item>
        </el-collapse>
        <el-empty v-if="conditionTypes.length === 0" description="暂无条件类型" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { componentSchemaApi, npcPresetApi, nodeTypeSchemaApi, conditionTypeSchemaApi } from '@/api/schema'

const activeTab = ref('components')
const loading = ref(false)

const componentSchemas = ref([])
const presets = ref([])
const nodeTypes = ref([])
const conditionTypes = ref([])

async function loadAll() {
  loading.value = true
  try {
    const [cs, ps, nt, ct] = await Promise.all([
      componentSchemaApi.list(),
      npcPresetApi.list(),
      nodeTypeSchemaApi.list(),
      conditionTypeSchemaApi.list(),
    ])
    componentSchemas.value = (cs.data.items || []).filter(i => !i.name.startsWith('_'))
    presets.value = ps.data.items || []
    nodeTypes.value = nt.data.items || []
    conditionTypes.value = ct.data.items || []
  } catch { /* 拦截器已处理 */ }
  finally { loading.value = false }
}

function formatJson(obj) {
  if (!obj) return '无 Schema 定义'
  return JSON.stringify(obj, null, 2)
}

function categoryColor(category) {
  if (category === 'composite') return 'primary'
  if (category === 'decorator') return 'warning'
  if (category === 'leaf') return 'success'
  return 'info'
}

onMounted(loadAll)
</script>

<style scoped>
.schema-manager {
  padding: 24px;
}
.schema-manager h2 {
  margin: 0 0 8px;
  color: #303133;
  font-size: 20px;
}
.schema-title {
  font-weight: 500;
}
.schema-meta {
  margin-bottom: 12px;
  font-size: 13px;
  color: #606266;
}
.schema-meta p {
  margin: 4px 0;
}
.schema-json {
  background: #f5f7fa;
  padding: 12px;
  border-radius: 4px;
  font-size: 12px;
  overflow-x: auto;
  max-height: 400px;
}
</style>
