package softdelete

import (
	"time"

	"github.com/duxweb/oro"
)

const extensionName = "softdelete"

type SoftDeleteFields struct {
	DeletedAt oro.Null[time.Time]
}

func (SoftDeleteFields) OroEmbeddedFields() {}

func (SoftDeleteFields) DefineOroFields(s *oro.SchemaBuilder) {
	Define(s)
}

func Define(s *oro.SchemaBuilder) {
	s.Field("DeletedAt").Column("deleted_at").Timestamp().SoftDelete()
}

type extension struct{}

func Extension() oro.Extension {
	return extension{}
}

func (extension) Name() string {
	return extensionName
}

func (extension) Install(db *oro.DB) error {
	return nil
}
