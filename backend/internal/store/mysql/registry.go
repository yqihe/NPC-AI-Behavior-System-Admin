package mysql

import "github.com/jmoiron/sqlx"

// Stores 聚合所有 MySQL store，新增模块加一行
type Stores struct {
	Field           *FieldStore
	FieldRef        *FieldRefStore
	Dict            *DictionaryStore
	Template        *TemplateStore
	EventType       *EventTypeStore
	EventTypeSchema *EventTypeSchemaStore
	FsmConfig       *FsmConfigStore
}

// NewStores 一次性初始化所有 store
func NewStores(db *sqlx.DB) *Stores {
	return &Stores{
		Field:           NewFieldStore(db),
		FieldRef:        NewFieldRefStore(db),
		Dict:            NewDictionaryStore(db),
		Template:        NewTemplateStore(db),
		EventType:       NewEventTypeStore(db),
		EventTypeSchema: NewEventTypeSchemaStore(db),
		FsmConfig:       NewFsmConfigStore(db),
	}
}
