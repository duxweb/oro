package oro

func TenantFieldsForSchema(fields []string, schema *ModelSchema) []string {
	if schema == nil || schema.NoTenant {
		return nil
	}
	if len(schema.TenantFields) > 0 {
		return append([]string(nil), schema.TenantFields...)
	}
	if len(fields) == 0 {
		return nil
	}
	for _, field := range fields {
		if _, ok := schema.FieldByGo[field]; !ok {
			return nil
		}
	}
	return append([]string(nil), fields...)
}
