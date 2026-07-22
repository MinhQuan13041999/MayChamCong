package postgres

// nullableUUID chuyển empty string thành nil để PostgreSQL nhận NULL thay vì lỗi UUID parse.
// Dùng cho các cột UUID nullable như department_id.
func nullableUUID(s string) interface{} {
	if s == "" || len(s) != 36 {
		return nil
	}
	return s
}

// nullableStr chuyển empty string thành nil để PostgreSQL lưu NULL.
// Dùng cho các cột có CHECK constraint như gender, hoặc các cột optional.
func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
