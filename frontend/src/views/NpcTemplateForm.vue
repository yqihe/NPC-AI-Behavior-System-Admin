<template>
  <div class="npc-template-form">
    <div class="form-header">
      <h2>{{ isEdit ? '编辑 NPC 模板' : '新建 NPC 模板' }}</h2>
      <el-button @click="goBack">返回列表</el-button>
    </div>

    <el-alert
      v-if="!isEdit"
      type="info"
      :closable="true"
      show-icon
      style="margin-bottom: 16px"
      title="如何创建 NPC？"
      description="① 输入模板名称 → ② 选择预设类型（决定 NPC 的复杂程度）→ ③ 勾选需要的功能组件 → ④ 展开每个组件填写参数 → ⑤ 保存。标记为「必选」的组件不能取消。"
    />

    <el-form
      ref="formRef"
      :model="formModel"
      :rules="nameFieldRules"
      label-position="top"
      style="max-width: 900px"
    >
      <!-- 名称 -->
      <el-form-item label="模板名称" prop="name">
        <el-input
          v-model="formModel.name"
          :disabled="isEdit"
          placeholder="请输入名称（如 wolf_common）"
        />
      </el-form-item>

      <!-- 预设选择 -->
      <el-form-item label="预设类型">
        <el-select
          v-model="presetName"
          :disabled="isEdit"
          placeholder="请选择预设"
          style="width: 100%"
          @change="onPresetChange"
        >
          <el-option
            v-for="p in presetList"
            :key="p.name"
            :label="`${p.config?.display_name || p.name} — ${p.config?.description || ''}`"
            :value="p.name"
          />
        </el-select>
      </el-form-item>

      <!-- 组件勾选区 -->
      <el-form-item v-if="presetName" label="组件选择">
        <div class="component-selector">
          <el-checkbox
            v-for="comp in allComponents"
            :key="comp.name"
            :model-value="enabledComponents.includes(comp.name)"
            :disabled="comp.isRequired"
            @change="(checked) => toggleComponent(comp.name, checked)"
          >
            {{ comp.displayName }} ({{ comp.name }})
            <el-tag v-if="comp.isRequired" type="danger" size="small">必选</el-tag>
            <el-tag v-else-if="comp.isDefault" type="success" size="small">默认</el-tag>
            <el-tag v-else size="small">可选</el-tag>
          </el-checkbox>
        </div>
      </el-form-item>

      <!-- 组件面板区（T3 实现） -->
      <div v-if="enabledComponents.length > 0" class="component-panels">
        <el-divider content-position="left">组件配置</el-divider>
        <el-collapse v-model="activePanels">
          <component-panel
            v-for="comp in enabledComponents"
            :key="comp"
            :component-name="comp"
            :display-name="getDisplayName(comp)"
            :schema="componentSchemas[comp] || null"
            :required="isRequiredComponent(comp)"
            v-model="componentData[comp]"
          />
        </el-collapse>
      </div>

      <!-- 保存按钮 -->
      <el-form-item v-if="presetName" style="margin-top: 24px">
        <el-button type="primary" :loading="saving" @click="handleSave">
          保存
        </el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import ComponentPanel from '@/components/ComponentPanel.vue'
import { npcTemplateApi } from '@/api/generic'
import { componentSchemaApi, npcPresetApi } from '@/api/schema'
import { createNameRules } from '@/utils/nameRules'

const route = useRoute()
const router = useRouter()

const routeName = route.params.name
const isEdit = computed(() => !!routeName && routeName !== 'new')

const formRef = ref(null)
const saving = ref(false)

const formModel = reactive({ name: '' })
const nameFieldRules = computed(() => ({
  name: isEdit.value
    ? [{ required: true, message: '名称不能为空', trigger: 'blur' }]
    : createNameRules({ listApi: npcTemplateApi.list, label: 'NPC 模板' }),
}))

// ========== 预设 ==========

const presetList = ref([])
const presetName = ref('')
const currentPreset = computed(() =>
  presetList.value.find(p => p.name === presetName.value)?.config || null
)

// ========== 组件 ==========

const enabledComponents = ref([])
const componentData = ref({})
const componentSchemas = ref({})
const allSchemaList = ref([])
const activePanels = ref([])

// 所有组件列表（根据预设分类：必选/默认/可选）
const allComponents = computed(() => {
  if (!currentPreset.value) return []

  const preset = currentPreset.value
  const required = preset.required_components || []
  const defaults = preset.default_components || []
  const optional = preset.optional_components || []

  const list = []
  for (const name of required) {
    list.push({ name, displayName: getDisplayName(name), isRequired: true, isDefault: false })
  }
  for (const name of defaults) {
    list.push({ name, displayName: getDisplayName(name), isRequired: false, isDefault: true })
  }
  for (const name of optional) {
    list.push({ name, displayName: getDisplayName(name), isRequired: false, isDefault: false })
  }
  return list
})

function getDisplayName(compName) {
  const schema = allSchemaList.value.find(s => s.name === compName)
  return schema?.config?.display_name || compName
}

function isRequiredComponent(compName) {
  return (currentPreset.value?.required_components || []).includes(compName)
}

// ========== 预设变更 ==========

function onPresetChange(newPreset) {
  const preset = presetList.value.find(p => p.name === newPreset)?.config
  if (!preset) return

  const required = preset.required_components || []
  const defaults = preset.default_components || []
  enabledComponents.value = [...required, ...defaults]

  // 初始化组件数据
  componentData.value = {}
  for (const name of enabledComponents.value) {
    componentData.value[name] = {}
  }

  // 加载组件 schema
  loadComponentSchemas(enabledComponents.value)

  activePanels.value = []
}

function toggleComponent(compName, checked) {
  if (checked) {
    if (!enabledComponents.value.includes(compName)) {
      enabledComponents.value.push(compName)
      componentData.value[compName] = {}
      loadComponentSchemas([compName])
    }
  } else {
    enabledComponents.value = enabledComponents.value.filter(c => c !== compName)
    delete componentData.value[compName]
  }
}

// ========== Schema 加载 ==========

async function loadComponentSchemas(names) {
  for (const name of names) {
    if (componentSchemas.value[name]) continue
    try {
      const res = await componentSchemaApi.get(name)
      componentSchemas.value[name] = res.data.config?.schema || null
    } catch {
      componentSchemas.value[name] = null
    }
  }
}

// ========== 保存 ==========

async function handleSave() {
  if (formRef.value) {
    try {
      await formRef.value.validate()
    } catch { return }
  }

  if (!formModel.name.trim()) {
    ElMessage.warning('请输入模板名称')
    return
  }
  if (!presetName.value) {
    ElMessage.warning('请选择预设类型')
    return
  }

  saving.value = true
  try {
    const components = {}
    for (const comp of enabledComponents.value) {
      components[comp] = componentData.value[comp] || {}
    }

    const payload = {
      name: formModel.name,
      config: {
        preset: presetName.value,
        components,
      },
    }

    if (isEdit.value) {
      await npcTemplateApi.update(routeName, payload)
      ElMessage.success('保存成功')
    } else {
      await npcTemplateApi.create(payload)
      ElMessage.success('创建成功')
    }
    goBack()
  } catch { /* 拦截器已处理 */ }
  finally { saving.value = false }
}

function goBack() {
  router.push('/npc-templates')
}

// ========== 初始化 ==========

onMounted(async () => {
  // 并行加载预设列表和组件 schema 列表
  try {
    const [presetsRes, schemasRes] = await Promise.all([
      npcPresetApi.list(),
      componentSchemaApi.list(),
    ])
    presetList.value = presetsRes.data.items || []
    allSchemaList.value = (schemasRes.data.items || []).filter(s => !s.name.startsWith('_'))
  } catch {
    ElMessage.error('加载预设或 Schema 失败')
  }

  // 编辑模式：加载已有数据
  if (isEdit.value) {
    try {
      const res = await npcTemplateApi.get(routeName)
      formModel.name = res.data.name
      const config = res.data.config || {}

      presetName.value = config.preset || ''
      const components = config.components || {}

      enabledComponents.value = Object.keys(components)
      componentData.value = { ...components }

      // 加载已启用组件的 schema
      await loadComponentSchemas(enabledComponents.value)
    } catch {
      ElMessage.error('加载模板数据失败')
      goBack()
    }
  }
})
</script>

<style scoped>
.npc-template-form {
  padding: 24px;
}
.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.form-header h2 {
  margin: 0;
  color: #303133;
  font-size: 20px;
}
.component-selector {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.component-panels {
  margin-top: 8px;
}
</style>
