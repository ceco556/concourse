package migration_test

import (
	"database/sql"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenHelper", func() {
	var (
		err         error
		db          *sql.DB
		lockDB      [lock.FactoryCount]*sql.DB
		lockFactory lock.LockFactory
		openHelper  *migration.OpenHelper
		fakeLogFunc = func(logger lager.Logger, id lock.LockID) {}
	)

	JustBeforeEach(func() {
		db, err = sql.Open("pgx", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		for i := 0; i < lock.FactoryCount; i++ {
			lockDB[i], err = sql.Open("pgx", postgresRunner.DataSourceName())
			Expect(err).NotTo(HaveOccurred())
		}
		lockFactory = lock.NewLockFactory(lockDB, fakeLogFunc, fakeLogFunc)
		openHelper = migration.NewOpenHelper("pgx", postgresRunner.DataSourceName(), lockFactory, nil, nil)
	})

	AfterEach(func() {
		_ = db.Close()
		for _, closer := range lockDB {
			closer.Close()
		}
	})

	Context("legacy migration_version table exists", func() {
		It("Fails if trying to upgrade from a migration_version < 189", func() {
			SetupMigrationVersionTableToExistAtVersion(db, 188)

			err = openHelper.MigrateToVersion(5000)

			Expect(err.Error()).To(Equal("must upgrade from db version 189 (concourse 3.6.0), current db version: 188"))

			_, err = db.Exec("SELECT version FROM migration_version")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Fails if trying to upgrade from a migration_version > 189", func() {
			SetupMigrationVersionTableToExistAtVersion(db, 190)

			err = openHelper.MigrateToVersion(5000)

			Expect(err.Error()).To(Equal("must upgrade from db version 189 (concourse 3.6.0), current db version: 190"))

			_, err = db.Exec("SELECT version FROM migration_version")
			Expect(err).NotTo(HaveOccurred())
		})

		It("Forces schema migration version to a known first version if migration_version is 189", func() {
			var initialSchemaVersion = 1510262030
			SetupMigrationVersionTableToExistAtVersion(db, 189)

			SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

			err = openHelper.MigrateToVersion(initialSchemaVersion)
			Expect(err).NotTo(HaveOccurred())

			ExpectDatabaseVersionToEqual(db, initialSchemaVersion, "schema_migrations")

			ExpectMigrationVersionTableNotToExist(db)

			ExpectToBeAbleToInsertData(db)
		})

		It("Runs migrator if migration_version table does not exist", func() {
			err = openHelper.MigrateToVersion(initialSchemaVersion)
			Expect(err).NotTo(HaveOccurred())

			ExpectDatabaseVersionToEqual(db, initialSchemaVersion, "migrations_history")

			ExpectMigrationVersionTableNotToExist(db)

			ExpectToBeAbleToInsertData(db)
		})
	})
})

func SetupMigrationVersionTableToExistAtVersion(db *sql.DB, version int) {
	_, err := db.Exec(`CREATE TABLE migration_version(version int)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO migration_version(version) VALUES($1)`, version)
	Expect(err).NotTo(HaveOccurred())
}

func ExpectMigrationVersionTableNotToExist(dbConn *sql.DB) {
	var exists string
	err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables where table_name = 'migration_version')").Scan(&exists)
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(Equal("false"))
}

func ExpectDatabaseVersionToEqual(db *sql.DB, version int, table string) {
	var dbVersion int
	query := "SELECT version from " + table + " LIMIT 1"
	err := db.QueryRow(query).Scan(&dbVersion)
	Expect(err).NotTo(HaveOccurred())
	Expect(dbVersion).To(Equal(version))
}
