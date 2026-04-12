package model

import "time"

type DimMarket struct {
	ID         uint      `gorm:"primaryKey"`
	MarketCode string    `gorm:"size:32;not null;uniqueIndex"`
	MarketName string    `gorm:"size:255;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (DimMarket) TableName() string { return "dim_market" }

type DimItem struct {
	ID        uint      `gorm:"primaryKey"`
	ItemCode  string    `gorm:"size:32;not null;uniqueIndex"`
	ItemName  string    `gorm:"size:255;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (DimItem) TableName() string { return "dim_item" }

type DimOrigin struct {
	ID         uint      `gorm:"primaryKey"`
	OriginCode string    `gorm:"size:32;not null;uniqueIndex"`
	OriginName string    `gorm:"size:255;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (DimOrigin) TableName() string { return "dim_origin" }

type DimGrade struct {
	ID          uint      `gorm:"primaryKey"`
	Grade       string    `gorm:"size:64;not null;uniqueIndex:ux_dim_grade_key,priority:1"`
	Class       string    `gorm:"size:64;not null;uniqueIndex:ux_dim_grade_key,priority:2"`
	ProductName string    `gorm:"size:255;not null;uniqueIndex:ux_dim_grade_key,priority:3"`
	UnitWeight  string    `gorm:"size:64;not null;uniqueIndex:ux_dim_grade_key,priority:4"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (DimGrade) TableName() string { return "dim_grade" }

type FactPricesDaily struct {
	ID           uint      `gorm:"primaryKey"`
	TradeDate    time.Time `gorm:"type:date;not null;index:idx_fact_trade_market,priority:1;index:idx_fact_trade_item_market,priority:1"`
	WeekdayJA    string    `gorm:"size:16;not null"`
	MarketID     uint      `gorm:"not null;index:idx_fact_trade_market,priority:2;index:idx_fact_trade_item_market,priority:3"`
	ItemID       uint      `gorm:"not null;index:idx_fact_item_trade,priority:1;index:idx_fact_trade_item_market,priority:2"`
	OriginID     uint      `gorm:"not null;index:idx_fact_origin_trade,priority:1"`
	GradeID      uint      `gorm:"not null"`
	ItemTotalTon *float64  `gorm:"type:numeric(14,3)"`
	ArrivalTon   *float64  `gorm:"type:numeric(14,3)"`
	PriceHighYen *int      `gorm:"type:integer"`
	PriceMidYen  *int      `gorm:"type:integer"`
	PriceLowYen  *int      `gorm:"type:integer"`
	TrendLabel   *string   `gorm:"size:64"`
	SourceFile   string    `gorm:"size:1024;not null;uniqueIndex:ux_fact_source_row,priority:1"`
	SourceRowNo  int       `gorm:"not null;uniqueIndex:ux_fact_source_row,priority:2"`
	IngestedAt   time.Time `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`

	Market DimMarket `gorm:"foreignKey:MarketID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Item   DimItem   `gorm:"foreignKey:ItemID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Origin DimOrigin `gorm:"foreignKey:OriginID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Grade  DimGrade  `gorm:"foreignKey:GradeID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (FactPricesDaily) TableName() string { return "fact_prices_daily" }

type IngestionRun struct {
	ID           uint      `gorm:"primaryKey"`
	RunType      string    `gorm:"size:64;not null;index"`
	StartedAt    time.Time `gorm:"not null"`
	FinishedAt   *time.Time
	Status       string          `gorm:"size:32;not null;index"`
	ErrorMessage *string         `gorm:"type:text"`
	CreatedAt    time.Time       `gorm:"not null"`
	UpdatedAt    time.Time       `gorm:"not null"`
	Files        []IngestionFile `gorm:"foreignKey:RunID;references:ID"`
}

func (IngestionRun) TableName() string { return "ingestion_runs" }

type IngestionFile struct {
	ID        uint      `gorm:"primaryKey"`
	RunID     uint      `gorm:"not null;index"`
	FilePath  string    `gorm:"size:2048;not null"`
	FileHash  string    `gorm:"size:128;not null"`
	RowsTotal int       `gorm:"not null;default:0"`
	RowsOK    int       `gorm:"not null;default:0"`
	RowsError int       `gorm:"not null;default:0"`
	Status    string    `gorm:"size:32;not null;index"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`

	Run IngestionRun `gorm:"foreignKey:RunID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (IngestionFile) TableName() string { return "ingestion_files" }
