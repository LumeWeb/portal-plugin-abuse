package models

type BlockReasonCount struct {
	BlockDate   string      `gorm:"column:block_date"`
	BlockReason BlockReason `gorm:"column:block_reason"`
	BlockCount  int64       `gorm:"column:block_count"`
}

func (BlockReasonCount) TableName() string {
	return "abuse_block_reason_counts"
}
