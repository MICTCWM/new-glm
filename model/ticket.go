package model

import (
	"gorm.io/gorm"
)

// Ticket status constants
const (
	TicketStatusOpen     = "open"
	TicketStatusClosed   = "closed"
	TicketStatusReplied  = "replied"
)

// Ticket represents a support ticket submitted by a user
type Ticket struct {
	Id          int            `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId      int            `json:"user_id" gorm:"index;not null"`
	Title       string         `json:"title" gorm:"type:varchar(255);not null"`
	Content     string         `json:"content" gorm:"type:text;not null"`
	Status      string         `json:"status" gorm:"type:varchar(32);default:'open';index"`
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`

	// Associations (not stored in DB, populated on query)
	Username string        `json:"username" gorm:"-:all"`
	Images   []TicketImage `json:"images" gorm:"foreignKey:TicketId"`
	Replies  []TicketReply `json:"replies" gorm:"foreignKey:TicketId"`
}

func (Ticket) TableName() string {
	return "tickets"
}

// TicketImage stores uploaded images for a ticket
type TicketImage struct {
	Id        int            `json:"id" gorm:"primaryKey;autoIncrement"`
	TicketId  int            `json:"ticket_id" gorm:"index;not null"`
	Filename  string         `json:"filename" gorm:"type:varchar(255);not null"`
	FilePath  string         `json:"file_path" gorm:"type:varchar(512);not null"`
	FileSize  int64          `json:"file_size" gorm:"type:bigint;default:0"`
	CreatedAt int64          `json:"created_at" gorm:"autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (TicketImage) TableName() string {
	return "ticket_images"
}

// TicketReply stores admin replies to a ticket
type TicketReply struct {
	Id        int            `json:"id" gorm:"primaryKey;autoIncrement"`
	TicketId  int            `json:"ticket_id" gorm:"index;not null"`
	UserId    int            `json:"user_id" gorm:"index;not null"`
	Content   string         `json:"content" gorm:"type:text;not null"`
	CreatedAt int64          `json:"created_at" gorm:"autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations (not stored in DB, populated on query)
	Username string `json:"username" gorm:"-:all"`
}

func (TicketReply) TableName() string {
	return "ticket_replies"
}
