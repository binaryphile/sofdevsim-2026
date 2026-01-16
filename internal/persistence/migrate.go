package persistence

// Migration framework for schema versioning.
// Currently at v1 - no migrations needed yet.
//
// When schema changes are needed:
// 1. Increment CurrentVersion in schema.go
// 2. Add a migration function here: migrateV1ToV2
// 3. Register it in the migrations map
// 4. Load() will automatically run the migration chain

// Migrator defines the interface for schema migrations.
type Migrator interface {
	FromVersion() int
	ToVersion() int
	Migrate(old *SaveFile) (*SaveFile, error)
}

// migrations maps source versions to their migrators.
// Example: migrations[1] = v1ToV2Migrator
var migrations = map[int]Migrator{}

// MigrateToLatest applies all necessary migrations to bring a save file
// to the current schema version.
func MigrateToLatest(saveFile *SaveFile) (*SaveFile, error) {
	current := saveFile

	for current.Version < CurrentVersion {
		migrator, ok := migrations[current.Version]
		if !ok {
			// No migration path - this shouldn't happen if migrations are registered correctly
			break
		}

		var err error
		current, err = migrator.Migrate(current)
		if err != nil {
			return nil, err
		}
	}

	return current, nil
}

// Example migration (commented out until needed):
//
// type v1ToV2Migrator struct{}
//
// func (m v1ToV2Migrator) FromVersion() int { return 1 }
// func (m v1ToV2Migrator) ToVersion() int   { return 2 }
//
// func (m v1ToV2Migrator) Migrate(old *SaveFile) (*SaveFile, error) {
//     // Transform v1 schema to v2
//     new := &SaveFile{
//         Version:   2,
//         Timestamp: old.Timestamp,
//         Name:      old.Name,
//         State:     old.State, // Transform as needed
//     }
//     return new, nil
// }
//
// func init() {
//     migrations[1] = v1ToV2Migrator{}
// }
