package esi

// UniverseType holds fully resolved EVE type data including group and category.
// It is returned by GetUniverseType, which internally chains three ESI calls:
//
//	GET /universe/types/{type_id}       → TypeName, GroupID
//	GET /universe/groups/{group_id}     → GroupName, CategoryID
//	GET /universe/categories/{cat_id}  → CategoryName
//
// Implemented in TASK-06.
type UniverseType struct {
	TypeID       int64
	TypeName     string
	GroupID      int64
	GroupName    string
	CategoryID   int64
	CategoryName string
}
