package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// db connections
	DB *gorm.DB

	ErrNotFound = gorm.ErrRecordNotFound
)

type DeletedAt = gorm.DeletedAt

func Init(addr string) error {
	var db *gorm.DB
	var err error

	if len(addr) == 0 {
		db, err = gorm.Open(sqlite.Open("turbo.db"), &gorm.Config{})
	} else {
		// postgress url format: "postgres://user:pass@localhost:5432/proxy"
		db, err = gorm.Open(postgres.Open(addr), &gorm.Config{})
	}
	if err != nil {
		return err
	}

	// set the DB
	DB = db

	return nil
}

func Migrate(vals ...interface{}) error {
	for _, v := range vals {
		// TODO: return error
		if err := DB.AutoMigrate(v); err != nil {
			return err
		}
	}
	return nil
}

func Model(val interface{}) *gorm.DB {
	return DB.Model(val)
}

// https://gorm.io/docs/create.html
func Create(val interface{}) *gorm.DB {
	return DB.Create(val)
}

// https://gorm.io/gen/clause.html
func Clauses(clause ...clause.Expression) *gorm.DB {
	return DB.Clauses(clause...)
}

// https://gorm.io/docs/delete.html
func Delete(val interface{}, conditions ...interface{}) *gorm.DB {
	return DB.Delete(val, conditions...)
}

// https://gorm.io/docs/update.html
func Update(val interface{}) *gorm.DB {
	return DB.Save(val)
}

func First(v interface{}, where ...interface{}) *gorm.DB {
	return DB.First(v, where...)
}

// https://gorm.io/docs/query.html#Retrieving-all-objects
func Find(q interface{}, args ...interface{}) *gorm.DB {
	return DB.Find(q, args...)
}

// https://gorm.io/docs/query.html#Order
func Order(v string) *gorm.DB {
	return DB.Order(v)
}

// https://gorm.io/docs/query.html#Conditions
func Where(q interface{}, args ...interface{}) *gorm.DB {
	return DB.Where(q, args...)
}

func Unscoped() *gorm.DB {
	return DB.Unscoped()
}
