package db

import (
	"japan_data_project/internal/domain/model"

	"gorm.io/gorm"
)

func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&model.DimMarket{},
		&model.DimItem{},
		&model.DimOrigin{},
		&model.DimGrade{},
		&model.IngestionRun{},
		&model.IngestionFile{},
		&model.FactPricesDaily{},
	)
}
