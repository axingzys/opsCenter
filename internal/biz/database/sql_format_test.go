package database

import "testing"

func TestFormatSQLBreaksMajorClauses(t *testing.T) {
	formatted := FormatSQL("select id, name from users where status = 'active' order by created_at desc limit 10")
	expected := "SELECT id, name\nFROM users\nWHERE status = 'active'\nORDER BY created_at DESC\nLIMIT 10"
	if formatted != expected {
		t.Fatalf("unexpected formatted SQL:\n%s", formatted)
	}
}

func TestFormatSQLPreservesCommentsAndStrings(t *testing.T) {
	formatted := FormatSQL("select '--keep me' as txt /* note */ from `user_table`")
	expected := "SELECT '--keep me' AS txt\n/* note */\nFROM `user_table`"
	if formatted != expected {
		t.Fatalf("unexpected formatted SQL:\n%s", formatted)
	}
}
