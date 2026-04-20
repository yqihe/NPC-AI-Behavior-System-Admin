package shared

import "fmt"

// Redis key 统一管理，所有 key 通过函数生成

const (
	prefixDict        = "dict:"
	prefixFieldList   = "fields:list:"
	prefixFieldDetail = "fields:detail:"
	prefixFieldLock   = "fields:lock:"

	prefixTemplateList   = "templates:list:"
	prefixTemplateDetail = "templates:detail:"
	prefixTemplateLock   = "templates:lock:"

	prefixEventTypeList   = "event_types:list:"
	prefixEventTypeDetail = "event_types:detail:"
	prefixEventTypeLock   = "event_types:lock:"

	prefixFsmConfigList   = "fsm_configs:list:"
	prefixFsmConfigDetail = "fsm_configs:detail:"
	prefixFsmConfigLock   = "fsm_configs:lock:"

	prefixFsmStateDictList   = "fsm_state_dicts:list:"
	prefixFsmStateDictDetail = "fsm_state_dicts:detail:"
	prefixFsmStateDictLock   = "fsm_state_dicts:lock:"

	prefixBtTreeList   = "bt_trees:list:"
	prefixBtTreeDetail = "bt_trees:detail:"
	prefixBtTreeLock   = "bt_trees:lock:"

	prefixBtNodeTypeList   = "bt_node_types:list:"
	prefixBtNodeTypeDetail = "bt_node_types:detail:"
	prefixBtNodeTypeLock   = "bt_node_types:lock:"

	prefixNPCList   = "npcs:list:"
	prefixNPCDetail = "npcs:detail:"
	prefixNPCLock   = "npcs:lock:"

	prefixRuntimeBbKeyList   = "runtime_bb_keys:list:"
	prefixRuntimeBbKeyDetail = "runtime_bb_keys:detail:"
	prefixRuntimeBbKeyLock   = "runtime_bb_keys:lock:"

	FieldListVersionKey        = "fields:list:version"
	TemplateListVersionKey     = "templates:list:version"
	EventTypeListVersionKey    = "event_types:list:version"
	FsmConfigListVersionKey    = "fsm_configs:list:version"
	FsmStateDictListVersionKey = "fsm_state_dicts:list:version"
	BtTreeListVersionKey       = "bt_trees:list:version"
	BtNodeTypeListVersionKey = "bt_node_types:list:version"

	NPCListVersionKey = "npcs:list:version"

	RuntimeBbKeyListVersionKey = "runtime_bb_keys:list:version"
)

// ── Dict ──

func DictKey(group string) string { return prefixDict + group }

// ── Field ──

func FieldListKey(version int64, name, typ, category, label string, enabled *bool, exposesBB *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%s:%s:%d:%d", prefixFieldList, version, name, typ, category, label, boolStr(enabled), boolStr(exposesBB), page, pageSize)
}
func FieldDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixFieldDetail, id) }
func FieldLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixFieldLock, id) }

// ── Template ──

func TemplateListKey(version int64, name, label string, enabled *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%d:%d", prefixTemplateList, version, name, label, boolStr(enabled), page, pageSize)
}
func TemplateDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixTemplateDetail, id) }
func TemplateLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixTemplateLock, id) }

// ── EventType ──

func EventTypeListKey(version int64, name, label, perceptionMode string, enabled *bool, page, pageSize int) string {
	pm := "*"
	if perceptionMode != "" {
		pm = perceptionMode
	}
	return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%d:%d", prefixEventTypeList, version, name, label, pm, boolStr(enabled), page, pageSize)
}
func EventTypeDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixEventTypeDetail, id) }
func EventTypeLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixEventTypeLock, id) }

// ── FsmConfig ──

func FsmConfigListKey(version int64, name, label string, enabled *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%d:%d", prefixFsmConfigList, version, name, label, boolStr(enabled), page, pageSize)
}
func FsmConfigDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixFsmConfigDetail, id) }
func FsmConfigLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixFsmConfigLock, id) }

// ── FsmStateDict ──

func FsmStateDictListKey(version int64, name, displayName, category string, enabled *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%d:%d", prefixFsmStateDictList, version, name, displayName, category, boolStr(enabled), page, pageSize)
}
func FsmStateDictDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixFsmStateDictDetail, id) }
func FsmStateDictLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixFsmStateDictLock, id) }

// ── BtTree ──

func BtTreeListKey(version int64, name, displayName string, enabled *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%d:%d", prefixBtTreeList, version, name, displayName, boolStr(enabled), page, pageSize)
}
func BtTreeDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixBtTreeDetail, id) }
func BtTreeLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixBtTreeLock, id) }

// ── BtNodeType ──

func BtNodeTypeListKey(version int64, typeName, label, category string, enabled *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%d:%d", prefixBtNodeTypeList, version, typeName, label, category, boolStr(enabled), page, pageSize)
}
func BtNodeTypeDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixBtNodeTypeDetail, id) }
func BtNodeTypeLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixBtNodeTypeLock, id) }

// ── NPC ──

func NPCListKey(version int64, label, name, templateName string, enabled *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%d:%d", prefixNPCList, version, label, name, templateName, boolStr(enabled), page, pageSize)
}
func NPCDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixNPCDetail, id) }
func NPCLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixNPCLock, id) }

// ── RuntimeBbKey ──

func RuntimeBbKeyListKey(version int64, name, label, typ, groupName string, enabled *bool, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%s:%d:%d", prefixRuntimeBbKeyList, version, name, label, typ, groupName, boolStr(enabled), page, pageSize)
}
func RuntimeBbKeyDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixRuntimeBbKeyDetail, id) }
func RuntimeBbKeyLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixRuntimeBbKeyLock, id) }

// boolStr 将 *bool 转为 key 分段
func boolStr(b *bool) string {
	if b == nil {
		return "*"
	}
	if *b {
		return "1"
	}
	return "0"
}
